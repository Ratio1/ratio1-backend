package service

import (
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
)

var euCountries = map[string]string{
	"AUT": "AT",
	"BEL": "BE",
	"BGR": "BG",
	"HRV": "HR",
	"CYP": "CY",
	"CZE": "CZ",
	"DNK": "DK",
	"EST": "EE",
	"FIN": "FI",
	"FRA": "FR",
	"DEU": "DE",
	"GRC": "EL",
	"HUN": "HU",
	"IRL": "IE",
	"ITA": "IT",
	"LVA": "LV",
	"LTU": "LT",
	"LUX": "LU",
	"MLT": "MT",
	"NLD": "NL",
	"POL": "PL",
	"PRT": "PT",
	"ROU": "RO",
	"SVK": "SK",
	"SVN": "SI",
	"ESP": "ES",
	"SWE": "SE",
}

type VIESResponse struct {
	XMLName xml.Name `xml:"result"`
	Vies    struct {
		UID               string `xml:"uid"`
		CountryCode       string `xml:"countryCode"`
		VATNumber         string `xml:"vatNumber"`
		Valid             bool   `xml:"valid"`
		TraderName        string `xml:"traderName"`
		TraderCompanyType string `xml:"traderCompanyType"`
		TraderAddress     string `xml:"traderAddress"`
		ID                string `xml:"id"`
		Date              string `xml:"date"`
		Source            string `xml:"source"`
	} `xml:"vies"`
	Error struct {
		Code        string `xml:"code"`
		Description string `xml:"description"`
		Details     string `xml:"detail"`
	} `xml:"error"`
}

func IsCompanyRegistered(countryCode, vat string) bool {
	twoLetter, ok := euCountries[strings.ToUpper(countryCode)]
	if !ok {
		return false
	}

	vat = strings.ToUpper(strings.TrimSpace(vat))
	if !strings.HasPrefix(vat, twoLetter) {
		vat = twoLetter + vat
	}
	url := config.Config.ViesApi.BaseUrl + "/get/vies/euvat/" + vat
	authURL := strings.Replace(url, "https://", "https://"+config.Config.ViesApi.User+":"+config.Config.ViesApi.Password+"@", 1)

	resp, err := http.Get(authURL)
	if err != nil {
		log.Error("error while calling api: " + err.Error())
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("error while reading response: " + err.Error())
		return false
	}

	var viesResp VIESResponse
	if err := xml.Unmarshal(body, &viesResp); err != nil {
		log.Error("error while parsing response: " + err.Error())
		return false
	}
	if viesResp.Error.Code != "" {
		log.Error("error code: " + viesResp.Error.Code + " with description: " + viesResp.Error.Description + " details: " + viesResp.Error.Details)
		return false
	}

	return viesResp.Vies.Valid
}
