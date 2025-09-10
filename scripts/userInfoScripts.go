package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
)

/*
MAKE sure to set all the needed variabels berfore running
*/

func main() {
	DBConnect()
	CreateAllUserInfo()
}

func CreateAllUserInfo() {
	kycs, err := getAllActiveKyc()
	if err != nil {
		fmt.Println("error on retrieving all active kycs: " + err.Error())
	}
	var allUserinfo []model.UserInfo
	for _, kyc := range kycs {
		if kyc.ApplicantId == "safeFalseApplicant" {
			continue
		}
		acc, ok, err := getAccountByEmail(kyc.Email)
		if err != nil {
			fmt.Println("error on retrieving account: " + err.Error() + " for user: " + kyc.Email)
			continue
		} else if !ok {
			fmt.Println("no account found  for user: " + kyc.Email)
			continue
		}

		userinfo, err := getClientInfos(kyc.ApplicantId, kyc.Uuid.String())
		if err != nil {
			fmt.Println("error for user: "+kyc.Email+" with applicant id : "+kyc.ApplicantId+" error: ", err.Error())
			continue
		}
		userinfo.BlockchainAddress = acc.Address
		userinfo.Email = kyc.Email
		allUserinfo = append(allUserinfo, *userinfo)
		time.Sleep(1 * time.Second) //TO not reach ratelimit from sumsub (300 req for 5 sec, 60req/sec)
	}
	olddata, _ := json.Marshal(allUserinfo)
	_ = os.WriteFile("allUserInfo.json", olddata, 0644)
}

func getClientInfos(applicantId, uuid string) (*model.UserInfo, error) {
	Url := "https://api.sumsub.com" + "/resources/applicants/" + applicantId + "/one"
	request, err := http.NewRequest("GET", Url, nil)
	if err != nil {
		return nil, errors.New("error while creating new http request: " + err.Error())
	}

	ts := fmt.Sprintf("%d", time.Now().Unix())
	message := ts + "GET" + "/resources/applicants/" + applicantId + "/one"

	request.Header.Add("X-App-Token", SumsubAppToken)
	request.Header.Add("X-App-Access-Sig", generateSignature(SumsubSecretKey, message))
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

func mapApplicantToInvoiceClient(app model.ApplicantProfile) *model.UserInfo {
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

	invoiceClient := model.UserInfo{
		Name:               name,
		Surname:            surname,
		CompanyName:        companyName,
		IdentificationCode: *identificationCode,
		Address:            addr,
		City:               city,
		State:              state,
		Country:            country,
		IsCompany:          app.Type == "company",
	}

	return &invoiceClient
}
