package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
)

type InitSessionResponse struct {
	Data  *GoodInitSessionResponse
	Error *BadInitSessionResponse
}

type GoodInitSessionResponse struct {
	Token  string `json:"token"`
	UserID string `json:"userId"`
}

type BadInitSessionResponse struct {
	Description   string `json:"description"`
	Code          int    `json:"code"`
	CorrelationId string `json:"correlationId"`
	ErrorCode     int    `json:"errorCode"`
	ErrorName     string `json:"errorName"`
}

func ProcessKycEvent(event model.SumsubEvent, kyc model.Kyc) error {
	layout := "2006-01-02 15:04:05.000"
	parsedTime, err := time.Parse(layout, event.CreatedAtMs)
	if err != nil {
		return errors.New("error while parsing time: " + err.Error())
	}

	if kyc.LastUpdated.After(parsedTime) { //Could be null
		return nil
	}

	kyc.LastUpdated = time.Now().UTC()

	switch event.Type {
	case model.ApplicantCreated:
		kyc.ApplicantId = event.ApplicantID

	case model.ApplicantReviewed:
		status := event.ReviewStatus
		if event.ReviewResult.ReviewAnswer == "RED" {
			if event.ReviewResult.ReviewRejectType == "FINAL" {
				status = model.StatusFinalRejected
				err = SendKycFinalRejectedEmail(kyc.Email)
				if err != nil {
					return errors.New("error while sending email: " + err.Error())
				}
			} else {
				status = model.StatusRejected
				err = SendStepRejectedEmail(kyc.Email)
				if err != nil {
					return errors.New("error while sending email: " + err.Error())
				}
			}
		} else if event.ReviewResult.ReviewAnswer == "GREEN" {
			status = model.StatusApproved
			err = SendKycConfirmedEmail(kyc.Email)
			if err != nil {
				return errors.New("error while sending email: " + err.Error())
			}
		}
		kyc.KycStatus = status

	case model.ApplicantActivated:
		kyc.IsActive = true

	case model.ApplicantDeactivated:
		kyc.IsActive = false

	case model.ApplicantDeleted:
		kyc.HasBeenDeleted = true

	case model.ApplicantReset:
		kyc.KycStatus = event.ReviewStatus
		kyc.HasBeenDeleted = false
		err = SendAccountResettedEmail(kyc.Email)
		if err != nil {
			return errors.New("error while sending email: " + err.Error())
		}

	default:
		status := event.ReviewStatus
		if event.ReviewResult.ReviewAnswer == "RED" {
			if event.ReviewResult.ReviewRejectType == "FINAL" {
				status = model.StatusFinalRejected
				err = SendKycFinalRejectedEmail(kyc.Email)
				if err != nil {
					return errors.New("error while sending email: " + err.Error())
				}
			} else {
				status = model.StatusRejected
				err = SendStepRejectedEmail(kyc.Email)
				if err != nil {
					return errors.New("error while sending email: " + err.Error())
				}
			}
		} else if event.ReviewResult.ReviewAnswer == "GREEN" {
			status = model.StatusApproved
			err = SendKycConfirmedEmail(kyc.Email)
			if err != nil {
				return errors.New("error while sending email: " + err.Error())
			}
		}
		kyc.KycStatus = status
	}

	err = storage.CreateOrUpdateKyc(&kyc)
	if err != nil {
		return errors.New("error while updateing kyc information on storage: " + err.Error())
	}

	return nil
}

func InitNewSession(uuid, level string) (*string, error) {
	payloadAsString := "{\"ttlInSecs\":600,\"levelName\":\"" + level + "\",\"userId\":\"" + uuid + "\"}"
	payload := strings.NewReader(payloadAsString)

	request, err := http.NewRequest("POST", config.Config.Sumsub.ApiUrl+config.Config.Sumsub.ApiEndpoint, payload)
	if err != nil {
		return nil, errors.New("error while creating new http request: " + err.Error())
	}

	ts := fmt.Sprintf("%d", time.Now().Unix())
	message := ts + "POST" + config.Config.Sumsub.ApiEndpoint + payloadAsString

	request.Header.Add("X-App-Token", config.Config.Sumsub.SumsubAppToken)
	request.Header.Add("X-App-Access-Sig", generateSignature(config.Config.Sumsub.SumsubSecretKey, message))
	request.Header.Add("X-App-Access-Ts", ts)
	request.Header.Add("Accept", "application/json")
	request.Header.Add("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, errors.New("error while executing http request: " + err.Error())
	}
	defer response.Body.Close()

	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.New("error while reading response body: " + err.Error())
	}

	if response.StatusCode != 200 {
		var badResponse BadInitSessionResponse
		err = json.Unmarshal(resBody, &badResponse)
		if err != nil {
			return nil, errors.New("could not parse response, error code" + strconv.Itoa(response.StatusCode) + " error: " + err.Error())
		}
		return nil, errors.New(badResponse.ErrorName + " " + strconv.Itoa(badResponse.ErrorCode) + " " + badResponse.Description)
	}

	var goodResponse GoodInitSessionResponse
	err = json.Unmarshal(resBody, &goodResponse)
	if err != nil {
		return nil, errors.New("error while parsing response: " + err.Error())
	}

	return &goodResponse.Token, nil
}

func GetClientInfos(applicantId, uuid string) (*model.InvoiceClient, error) {
	Url := "https://api.sumsub.com" + "/resources/applicants/" + applicantId + "/one"
	request, err := http.NewRequest("GET", Url, nil)
	if err != nil {
		return nil, errors.New("error while creating new http request: " + err.Error())
	}

	ts := fmt.Sprintf("%d", time.Now().Unix())
	message := ts + "GET" + "/resources/applicants/" + applicantId + "/one"

	request.Header.Add("X-App-Token", config.Config.Sumsub.SumsubAppToken)
	request.Header.Add("X-App-Access-Sig", generateSignature(config.Config.Sumsub.SumsubSecretKey, message))
	request.Header.Add("X-App-Access-Ts", ts)
	request.Header.Add("Accept", "application/json")
	request.Header.Add("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, errors.New("error while executing http request: " + err.Error())
	}
	defer response.Body.Close()

	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.New("error while reading response body: " + err.Error())
	}
	var apiResult model.ApplicantProfile
	err = json.Unmarshal(resBody, &apiResult)
	if err != nil {
		return nil, errors.New("could not parse response")
	}

	if apiResult.ExternalUserID != uuid {
		return nil, errors.New("user received is not user expected")
	}

	return mapApplicantToInvoiceClient(apiResult), nil
}

func generateSignature(secret, message string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func mapApplicantToInvoiceClient(app model.ApplicantProfile) *model.InvoiceClient {
	var name, surname, companyName, identificationCode *string
	var addr, city, state, country string
	if app.Type == "company" && app.FixedInfo.CompanyInfo != nil {
		companyName = &app.FixedInfo.CompanyInfo.CompanyName
		identificationCode = &app.FixedInfo.CompanyInfo.TaxID
		addr = app.FixedInfo.CompanyInfo.Address.Street
		city = app.FixedInfo.CompanyInfo.Address.Town
		state = app.FixedInfo.CompanyInfo.Address.State
		country = app.FixedInfo.CompanyInfo.Address.Country

	} else {
		identificationCode = &app.FixedInfo.Tin
		name = &app.FixedInfo.FirstName
		surname = &app.FixedInfo.LastName
		if len(app.FixedInfo.Addresses) > 0 {
			addr = app.FixedInfo.Addresses[0].Street
			city = app.FixedInfo.Addresses[0].Town
			state = app.FixedInfo.Addresses[0].State
			country = app.FixedInfo.Addresses[0].Country
		}

	}

	invoiceClient := model.InvoiceClient{
		Uuid:               nil,
		Name:               name,
		Surname:            surname,
		CompanyName:        companyName,
		UserEmail:          nil,
		IdentificationCode: *identificationCode,
		Address:            addr,
		City:               city,
		State:              state,
		Country:            country,
		IsCompany:          app.Type == "company",
		Status:             nil,
		InvoiceUrl:         nil,
		InvoiceNumber:      nil,
		TxHash:             nil,
		BlockNumber:        nil,
	}

	return &invoiceClient
}
