package service

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/ratio1abi"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func ElaborateInvoices() {
	reportError := func(message string, err error, fields ...ErrorEmailField) {
		allFields := []ErrorEmailField{
			{Name: "Process", Value: "ElaborateInvoices"},
		}
		allFields = append(allFields, fields...)
		notifyError(message, err, allFields...)
	}

	latestSeenBlock, _, err := storage.GetLatestInvoiceBlock()
	if err != nil {
		reportError("Failed to retrieve latest invoice block from storage", err)
		return
	}

	events, err := fetchEvents(latestSeenBlock)
	if err != nil {
		reportError("Failed to fetch invoice events", err)
		return
	}

	var auth model.AuthRequest
	err = process.HttpPostWithUrlEncoded(config.Config.Oblio.AuthUrl, config.Config.Oblio.ClientSecret, &auth)
	if err != nil {
		reportError("Failed to authenticate with Oblio", err)
		return
	}

	for _, event := range events {
		invoice, found, err := storage.GetInvoiceByID(event.InvoiceID)
		if err != nil {
			reportError(
				"Failed to retrieve invoice from storage",
				err,
				ErrorEmailField{Name: "InvoiceID", Value: event.InvoiceID},
				ErrorEmailField{Name: "TransactionHash", Value: event.TxHash},
			)
			continue
		}
		if !found {
			reportError(
				"Invoice event received but invoice was not found in storage",
				errors.New("invoice not found"),
				ErrorEmailField{Name: "InvoiceID", Value: event.InvoiceID},
				ErrorEmailField{Name: "TransactionHash", Value: event.TxHash},
			)
			continue
		}
		if *invoice.Status != model.InvoiceStatusPending {
			fmt.Println("Invoice already processed: " + event.InvoiceID)
			continue
		}

		url, invoiceNumber, err := generateInvoice(*invoice, event, auth)
		if err != nil {
			reportError(
				"Failed to generate invoice on Oblio",
				err,
				ErrorEmailField{Name: "InvoiceID", Value: event.InvoiceID},
				ErrorEmailField{Name: "TransactionHash", Value: event.TxHash},
			)
			continue
		}

		status := model.InvoiceStatusPaid
		invoice.InvoiceNumber = &invoiceNumber
		invoice.InvoiceUrl = &url
		invoice.Status = &status
		invoice.TxHash = &event.TxHash
		invoice.BlockNumber = &event.BlockNumber
		invoice.NumLicenses = &event.NumLicenses
		invoice.UnitUsdPrice = &event.UnitUsdPrice

		err = storage.UpdateInvoice(invoice)
		if err != nil {
			reportError(
				"Invoice generated on Oblio but failed to persist in storage",
				err,
				ErrorEmailField{Name: "InvoiceID", Value: event.InvoiceID},
				ErrorEmailField{Name: "InvoiceNumber", Value: invoiceNumber},
				ErrorEmailField{Name: "InvoiceURL", Value: url},
				ErrorEmailField{Name: "TransactionHash", Value: event.TxHash},
			)
		}

		allEmails := append(config.Config.InvoiceEmail, config.Config.ErrorEmail...)
		for _, recipient := range allEmails {
			recipient = strings.TrimSpace(recipient)
			if recipient == "" {
				continue
			}
			urlCopy := url
			invoiceNumberCopy := invoiceNumber
			EnqueueEmailTask(EmailTask{
				Name: "send_buy_license_email",
				Execute: func() error {
					return SendBuyLicenseEmail(recipient, urlCopy, invoiceNumberCopy)
				},
			})
		}
	}
}

func fetchEvents(latestSeenBlock *int64) ([]model.Event, error) {
	contractAddress := common.HexToAddress(config.Config.NDContractAddress)

	var fromBlock *big.Int
	if latestSeenBlock != nil {
		fromBlock = big.NewInt(*latestSeenBlock)
	}

	eventSignatureAsBytes := []byte(ratio1abi.OblioEventSignature)
	eventHash := crypto.Keccak256Hash(eventSignatureAsBytes)

	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{eventHash}},
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return nil, errors.New("error while dialing client: " + err.Error())
	}
	defer client.Close()

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		return nil, errors.New("error while filtering logs: " + err.Error())
	}

	var events []model.Event
	decodeErrors := 0
	var firstDecodeError error
	for _, vLog := range logs {
		event, err := decodeLogs(vLog)
		if err != nil {
			fmt.Println("error while decoding logs: " + err.Error())
			decodeErrors++
			if firstDecodeError == nil {
				firstDecodeError = err
			}
			continue
		}
		events = append(events, *event)
	}
	if decodeErrors > 0 {
		notifyError(
			"Failed to decode one or more Oblio logs",
			firstDecodeError,
			ErrorEmailField{Name: "Process", Value: "fetchEvents"},
			ErrorEmailField{Name: "DecodeErrorsCount", Value: intField(decodeErrors)},
		)
	}

	return events, nil
}

func decodeLogs(vLog types.Log) (*model.Event, error) {
	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.OblioLicensesCreatedAbi))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	event := struct {
		TokenCount   *big.Int
		UnitUsdPrice *big.Int
		TotalR1Spent *big.Int
	}{}

	err = parsedABI.UnpackIntoInterface(&event, "LicensesCreated", vLog.Data)
	if err != nil {
		return nil, errors.New("error while unpacking interface: " + err.Error())
	}

	to := common.HexToAddress(vLog.Topics[1].Hex())
	invoiceUuid := vLog.Topics[2].Hex()

	invoiceUuidAsBytes, err := hex.DecodeString(invoiceUuid[2:])
	if err != nil {
		return nil, errors.New("error while decoding invoice uuid: " + err.Error())
	}

	oneToken := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)

	totalR1Float := new(big.Float).SetInt(event.TotalR1Spent)
	oneTokenFloat := new(big.Float).SetInt(oneToken)

	tokenAmount := new(big.Float).Quo(totalR1Float, oneTokenFloat)
	tokenAmountAsFloat, _ := tokenAmount.Float64()
	result := model.Event{
		Address:      to.Hex(),
		InvoiceID:    string(invoiceUuidAsBytes),
		NumLicenses:  int(event.TokenCount.Int64()),
		UnitUsdPrice: int(event.UnitUsdPrice.Int64()),
		TokenPaid:    tokenAmountAsFloat,
		TxHash:       vLog.TxHash.Hex(),
		BlockNumber:  int64(vLog.BlockNumber),
	}
	return &result, nil
}

func generateInvoice(invoiceData model.InvoiceClient, invoiceRequest model.Event, auth model.AuthRequest) (url string, invoiceNumber string, err error) {
	var headers = []process.HttpHeaderPair{
		{Key: "Authorization",
			Value: "Bearer " + auth.AccessToken},
	}

	var name string
	if invoiceData.IsCompany {
		name = *invoiceData.CompanyName
	} else {
		name = *invoiceData.Name + " " + *invoiceData.Surname
	}

	var client = model.OblioInvoiceClient{
		CIF:     invoiceData.IdentificationCode,
		Name:    name,
		Address: invoiceData.Address,
		State:   invoiceData.State,
		City:    invoiceData.City,
		Country: GetRoNameForISOCode(invoiceData.Country),
		Email:   *invoiceData.UserEmail,
	}

	product := model.InvoiceProduct{
		Name:          "R1 Node License for the operation of a R1 Edge Node software on a beneficiary provided hardware under the terms of Ratio1.ai EULA and T&C",
		Price:         int64(invoiceRequest.UnitUsdPrice),
		Quantity:      int64(invoiceRequest.NumLicenses),
		MeasuringUnit: "unit",
		Currency:      "USD",
		VatPercentage: 21,
		VatIncluded:   0, // 0 = false, 1 = true
	}

	if invoiceData.IsCompany && invoiceData.Country != model.ROU_ID {
		if invoiceData.ReverseCharge {
			product.VatPercentage = 0
			product.VatName = "Taxare inversa"
		} else if !invoiceData.IsUe {
			product.VatPercentage = 0
			product.VatName = "Scutita"
		}
	} else if !invoiceData.IsCompany && invoiceData.Country != model.ROU_ID {
		vat := GetEuVatPercentage(invoiceData.Country)
		if vat != nil {
			vatAsFloat := float64(*vat) / 100
			product.VatPercentage = vatAsFloat
			product.VatName = "VAT " + invoiceData.Country
		} else {
			product.VatPercentage = 0
			product.VatName = "Neimpozabil in Romania conform art. 278"
		}
	}
	percentage := float64(100) + product.VatPercentage                                                     // 100% + VAT percentage
	tokenPaidWithoutVat := (invoiceRequest.TokenPaid * 100) / percentage                                   // Calculate the token amount without VAT
	pricePerToken := float64(invoiceRequest.UnitUsdPrice*invoiceRequest.NumLicenses) / tokenPaidWithoutVat // Calculate the price per token in USD

	var mentions = "Amount paid: " + strconv.FormatFloat(invoiceRequest.TokenPaid, 'f', 2, 64) + " R1 tokens\nExchange rate: 1 R1 = " + strconv.FormatFloat(pricePerToken, 'f', 4, 64) + " USD" + "\nTxHash: " + invoiceRequest.TxHash

	var collect = model.InvoiceCollect{
		Type:           "Alta incasare banca",
		DocumentNumber: invoiceRequest.TxHash,
	}

	var invoice = model.InvoiceRequest{
		CIF:        model.InvoiceCif,
		SeriesName: model.InvoiceSeriesName,
		Language:   "EN",
		Currency:   "USD",
		Mentions:   mentions,
		Number:     1,
		SendEmail:  0,
		Client:     client,
		Product:    []model.InvoiceProduct{product},
		Collect:    collect,
	}

	if invoiceData.Country == model.ROU_ID {
		invoice.Language = "RO"
		invoice.SeriesName = model.InvoiceROUSeriesName
	}

	var oblioResponse model.OblioInvoiceResponse
	err = process.HttpPost(config.Config.Oblio.InvoiceUrl, invoice, &oblioResponse, headers...)

	if err != nil {
		return "", "", errors.New("error while doing http request: " + err.Error())
	}

	if oblioResponse.Status != 200 {
		return "", "", errors.New("error: " + strconv.Itoa(int(oblioResponse.Status)) + " " + oblioResponse.StatusMessage)
	}

	return oblioResponse.Data.Link, oblioResponse.Data.Number, nil
}
