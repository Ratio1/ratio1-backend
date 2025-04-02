package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

const COUNTRY_CODE = "ROU"
const (
	launchpadBaseEndpoint = "/license"
	mintTokensEndpoint    = "/buy"
)

type BuyLicenseResponse struct {
	Signature      string `json:"signature"`
	USDLimitAmount int    `json:"usdLimitAmount"`
	VatPercentage  int64  `json:"vatPercentage"`
	Uuid           string `json:"uuid"`
}

type launchpadHandler struct{}

func NewLaunchpadHandler(groupHandler *groupHandler) {
	h := &launchpadHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: mintTokensEndpoint, HandlerFunc: h.buyLicense},
	}

	endpointGroupHandler := EndpointGroupHandler{
		Root:             launchpadBaseEndpoint,
		Middleware:       []gin.HandlerFunc{middleware.Authorization(config.Config.Jwt.Secret)},
		EndpointHandlers: endpoints,
	}

	groupHandler.AddEndpointGroupHandler(endpointGroupHandler)
}

func (h *launchpadHandler) buyLicense(c *gin.Context) { //TODO nuovo endpoint verifica kyc con firma dell'address utente +nodo
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	address, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	client := &model.InvoiceClient{}
	kyc := &model.Kyc{}
	acc := &model.Account{}
	if config.Config.Api.DevTesting {
		kyc.ApplicantType = "individual"
		acc.Email = new(string)
	} else {
		acc, err = service.GetOrCreateAccount(address)
		if err != nil {
			log.Error("error while retrieving account information: " + err.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
			return
		} else if acc == nil {
			log.Error("error while retrieving account information: account does not exist")
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, service.ErrorAccountNotFound.Error())
			return
		}
		if acc.Email == nil || *acc.Email == "" {
			log.Error("email not found")
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, errors.New("email not found").Error())
			return
		}

		if acc.IsBlacklisted {
			if acc.BlacklistedReason != nil {
				log.Error("account: " + address + " is blacklisted with reason: " + *acc.BlacklistedReason)
				model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "account is blacklisted with reason:"+*acc.BlacklistedReason)
				return
			} else {
				log.Error("account: " + address + " is blacklisted!")
				model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "account is blacklisted")
				return
			}
		}

		kyc, found, err := storage.GetKycByEmail(*acc.Email)
		if err != nil {
			log.Error("error while retrieving kyc information from storage: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		} else if !found {
			log.Error("kyc not found in storage")
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "user email not found")
			return
		}

		if !kyc.IsActive || kyc.KycStatus != model.StatusApproved || kyc.HasBeenDeleted {
			log.Error(service.ErrorKycNotCompleted.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, service.ErrorKycNotCompleted.Error())
			return
		}

		if kyc.ApplicantType == "" {
			log.Error("empty applicant type found")
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "empty applicant type found")
			return
		}

		client, err := service.GetClientInfos(kyc.ApplicantId, kyc.Uuid.String())
		if err != nil {
			log.Error("error while retrieving Client information: " + err.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
			return
		} else if client == nil {
			log.Error("nil client returned from sumsub api")
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "nil client returned from sumsub api")
			return
		}

		err = validateData(*client)
		if err != nil {
			log.Error("error while validating client data: " + err.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
			return
		}
	}

	vatPercentage := int64(19)
	if client.IsCompany && client.Country != COUNTRY_CODE {
		client.ReverseCharge, client.IsUe = service.IsCompanyRegisteredAndUE(client.Country, client.IdentificationCode)
		vatPercentage = 0
	} else if !client.IsCompany && client.Country != model.ROU_ID {
		vat := service.GetEuVatPercentage(client.Country)
		if vat != nil {
			vatPercentage = *vat
		} else {
			vatPercentage = 0
		}
	}

	newUuid := uuid.New()
	newString := strings.ReplaceAll(newUuid.String(), "-", "")
	status := model.InvoiceStatusPending
	client.Uuid = &newString
	client.Status = &status
	client.UserEmail = acc.Email

	err = storage.CreateInvoice(client)
	if err != nil {
		log.Error("error while creating invoice in storage: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	var amount int
	if kyc.ApplicantType == model.BusinessCustomer {
		amount = config.Config.BuyLimitUSD.Company
	} else if kyc.ApplicantType == model.IndividualCustomer {
		amount = config.Config.BuyLimitUSD.Individual
	} else {
		log.Error("invalid applicant type: " + kyc.ApplicantType)
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "invalid applicant type: "+kyc.ApplicantType)
		return
	}

	signature, err := service.NewBuyLicenseTxTemplate(address, *client.Uuid, amount, vatPercentage)
	if err != nil {
		log.Error("error while trying to sign message: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	response := BuyLicenseResponse{
		Signature:      signature,
		USDLimitAmount: amount,
		VatPercentage:  vatPercentage,
		Uuid:           *client.Uuid,
	}

	model.JsonResponse(c, http.StatusOK, response, nodeAddress, "")
}

func validateData(client model.InvoiceClient) error {
	if client.IsCompany && client.CompanyName == nil {
		return errors.New("company name must be provided")
	} else if !client.IsCompany && (client.Surname == nil || client.Name == nil) {
		return errors.New("name and surname must be provided")
	}

	if client.Name == nil && client.Surname == nil && client.CompanyName == nil {
		return errors.New("name and surname or company name must be provided")
	}

	if client.IdentificationCode == "" {
		return errors.New("identification code must be provided")
	}

	if client.Address == "" {
		return errors.New("address must be provided")
	}

	if client.State == "" {
		return errors.New("state must be provided")
	}

	if client.City == "" {
		return errors.New("city must be provided")
	}

	if client.Country == "" {
		return errors.New("country must be provided")
	}

	return nil
}
