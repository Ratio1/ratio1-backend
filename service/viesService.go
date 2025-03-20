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

var euCountriesVat = map[string]float64{
	"AUT": 20,
	"BEL": 21,
	"BGR": 20, //GREAT BRITAIN
	"HRV": 25,
	"CYP": 19,
	"CZE": 21,
	"DNK": 25,
	"EST": 22,
	"FIN": 26,
	"FRA": 20,
	"DEU": 19,
	"GRC": 24,
	"HUN": 27,
	"IRL": 23,
	"ITA": 22,
	"LVA": 21,
	"LTU": 21,
	"LUX": 17,
	"MLT": 18,
	"NLD": 21,
	"POL": 23,
	"PRT": 23,
	"ROU": 19,
	"SVK": 23,
	"SVN": 22,
	"ESP": 21,
	"SWE": 25,
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

func IsCompanyRegisteredAndUE(countryCode, vat string) (idRegistered bool, isUe bool) {
	twoLetter, ok := euCountries[strings.ToUpper(countryCode)]
	if !ok {
		return false, false
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
		return false, true
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("error while reading response: " + err.Error())
		return false, true
	}

	var viesResp VIESResponse
	if err := xml.Unmarshal(body, &viesResp); err != nil {
		log.Error("error while parsing response: " + err.Error())
		return false, true
	}
	if viesResp.Error.Code != "" {
		log.Error("error code: " + viesResp.Error.Code + " with description: " + viesResp.Error.Description + " details: " + viesResp.Error.Details)
		return false, true
	}

	return viesResp.Vies.Valid, true
}

func GetEuVatPercentage(countryCode string) *float64 {
	vat, ok := euCountriesVat[strings.ToUpper(countryCode)]
	if !ok {
		return nil
	}
	return &vat
}
