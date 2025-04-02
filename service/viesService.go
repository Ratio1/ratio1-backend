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

var euCountriesVat = map[string]int64{
	"AUT": 20_00,
	"BEL": 21_00,
	"BGR": 20_00,
	"HRV": 25_00,
	"CYP": 19_00,
	"CZE": 21_00,
	"DNK": 25_00,
	"EST": 22_00,
	"FIN": 26_00,
	"FRA": 20_00,
	"DEU": 19_00,
	"GRC": 24_00,
	"HUN": 27_00,
	"IRL": 23_00,
	"ITA": 22_00,
	"LVA": 21_00,
	"LTU": 21_00,
	"LUX": 17_00,
	"MLT": 18_00,
	"NLD": 21_00,
	"POL": 23_00,
	"PRT": 23_00,
	"ROU": 19_00,
	"SVK": 23_00,
	"SVN": 22_00,
	"ESP": 21_00,
	"SWE": 25_00,
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

func GetEuVatPercentage(countryCode string) *int64 {
	vat, ok := euCountriesVat[strings.ToUpper(countryCode)]
	if !ok {
		return nil
	}
	return &vat
}
