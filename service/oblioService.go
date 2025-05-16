package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func ElaborateInvoices() {
	latestSeenBlock, _, err := storage.GetLatestInvoiceBlock()
	if err != nil {
		log.Error("Error receiving latest block from database: " + err.Error())
		return
	}

	events, err := fetchEvents(latestSeenBlock)
	if err != nil {
		log.Error("Error fetching events: " + err.Error())
		return
	}

	for _, event := range events {
		invoice, found, err := storage.GetInvoiceByID(event.InvoiceID)
		if err != nil {
			log.Error("Error retrieving invoice infromation from storage: " + err.Error())
			continue
		}
		if !found {
			log.Error("Invoice not found in storage: " + event.InvoiceID)
			continue
		}
		if *invoice.Status != model.InvoiceStatusPending {
			log.Error("Invoice already processed: " + event.InvoiceID)
			continue
		}

		url, invoiceNumber, err := generateInvoice(*invoice, event)
		if err != nil {
			log.Error("Error generating invoice: " + err.Error())
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
			log.Error("Error updating invoices in storage: " + err.Error())
		}

		time.Sleep(1 * time.Second)
	}
}

func fetchEvents(latestSeenBlock *int64) ([]model.Event, error) {
	contractAddress := common.HexToAddress(config.Config.NDContractAddress)

	var fromBlock *big.Int
	if latestSeenBlock != nil {
		fromBlock = big.NewInt(*latestSeenBlock)
	}

	eventSignatureAsBytes := []byte(config.Config.Oblio.EventSignature)
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
	for _, vLog := range logs {
		event, err := decodeLogs(vLog)
		if err != nil {
			log.Error("error while decoding logs: " + err.Error())
			continue
		}
		events = append(events, *event)
	}

	return events, nil
}

func decodeLogs(vLog types.Log) (*model.Event, error) {
	parsedABI, err := abi.JSON(strings.NewReader(config.Config.Oblio.Abi))
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

func generateInvoice(invoiceData model.InvoiceClient, invoiceRequest model.Event) (url string, invoiceNumber string, err error) {
	var auth model.AuthRequest
	err = process.HttpPostWithUrlEncoded(config.Config.Oblio.AuthUrl, config.Config.Oblio.ClientSecret, &auth)
	if err != nil {
		return "", "", errors.New("error while doing auth http request: " + err.Error())
	}

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

	pricePerToken := float64(invoiceRequest.UnitUsdPrice) / invoiceRequest.TokenPaid

	var mentions = "Amount paid: " + strconv.FormatFloat(invoiceRequest.TokenPaid, 'f', 2, 64) + " R1 tokens\nExchange rate: 1 R1 = " + strconv.FormatFloat(pricePerToken, 'f', 4, 64) + " USD" + "\nTxHash: " + invoiceRequest.TxHash

	var client = model.OblioInvoiceClient{
		CIF:     invoiceData.IdentificationCode,
		Name:    name,
		Address: invoiceData.Address,
		State:   invoiceData.State,
		City:    invoiceData.City,
		Country: invoiceData.Country,
		Email:   *invoiceData.UserEmail,
	}

	product := model.InvoiceProduct{
		Name:          "License",
		Price:         int64(invoiceRequest.UnitUsdPrice),
		Quantity:      int64(invoiceRequest.NumLicenses),
		MeasuringUnit: "unit",
		Currency:      "USD",
		VatPercentage: 19,
		VatIncluded:   0,
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
		SendEmail:  1,
		Client:     client,
		Product:    []model.InvoiceProduct{product},
		Collect:    collect,
	}

	if invoiceData.Country == model.ROU_ID {
		invoice.Language = "RO"
		invoice.SeriesName = model.InvoiceROUSeriesName
	}

	data, _ := json.Marshal(invoice)
	fmt.Println(string(data))

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
