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

var countriesName = map[string]string{
	"ABW": "Aruba",
	"AFG": "Afganistan",
	"AGO": "Angola",
	"AIA": "Anguilla",
	"ALB": "Albania",
	"AND": "Andorra",
	"ARE": "Emiratele Arabe Unite",
	"ARG": "Argentina",
	"ARM": "Armenia",
	"ATF": "Teritoriile Australe si Antarctice Franceze",
	"ATG": "Antigua si Barbuda",
	"AUS": "Australia",
	"AUT": "Austria",
	"AZE": "Azerbaidjan",
	"BDI": "Burundi",
	"BEL": "Belgia",
	"BEN": "Benin",
	"BFA": "Burkina Faso",
	"BGD": "Bangladesh",
	"BGR": "Bulgaria",
	"BHR": "Bahrain",
	"BHS": "Bahamas",
	"BIH": "Bosnia si Hertegovina",
	"BLM": "Saint-Barthelemy",
	"BLR": "Belarus",
	"BLZ": "Belize",
	"BMU": "Bermuda",
	"BOL": "Bolivia",
	"BRA": "Brazilia",
	"BRB": "Barbados",
	"BRN": "Brunei",
	"BTN": "Bhutan",
	"BWA": "Botswana",
	"CAF": "Republica Centrafricana",
	"CAN": "Canada",
	"CHE": "Elvetia",
	"CHL": "Chile",
	"CHN": "China",
	"CIV": "Cote d’Ivoire",
	"CMR": "Camerun",
	"COD": "Republica Democratica Congo",
	"COG": "Congo",
	"COK": "Insulele Cook",
	"COL": "Columbia",
	"COM": "Comore",
	"CPV": "Capul Verde",
	"CRI": "Costa Rica",
	"CUB": "Cuba",
	"CUW": "Curacao",
	"CYM": "Insulele Cayman",
	"CYP": "Cipru",
	"CZE": "Cehia",
	"DEU": "Germania",
	"DJI": "Djibouti",
	"DMA": "Dominica",
	"DNK": "Danemarca",
	"DOM": "Republica Dominicana",
	"DZA": "Algeria",
	"ECU": "Ecuador",
	"EGY": "Egipt",
	"ERI": "Eritreea",
	"ESH": "Sahara Occidentala",
	"ESP": "Spania",
	"EST": "Estonia",
	"ETH": "Etiopia",
	"FIN": "Finlanda",
	"FJI": "Fiji",
	"FLK": "Insulele Falkland",
	"FRA": "Franta",
	"FRO": "Insulele Feroe",
	"FSM": "Micronezia",
	"GAB": "Gabon",
	"GEO": "Georgia",
	"GGY": "Guernsey",
	"GHA": "Ghana",
	"GIB": "Gibraltar",
	"GIN": "Guineea",
	"GMB": "Gambia",
	"GNB": "Guineea-Bissau",
	"GNQ": "Guineea Ecuatoriala",
	"GRC": "Grecia",
	"GRD": "Grenada",
	"GRL": "Groenlanda",
	"GTM": "Guatemala",
	"GUY": "Guyana",
	"HND": "Honduras",
	"HRV": "Croatia",
	"HTI": "Haiti",
	"HUN": "Ungaria",
	"IDN": "Indonezie",
	"IMN": "Insula Man",
	"IND": "India",
	"IRL": "Irlanda",
	"IRN": "Iran",
	"IRQ": "Irak",
	"ISL": "Islanda",
	"ISR": "Israel",
	"ITA": "Italia",
	"JAM": "Jamaica",
	"JEY": "Jersey",
	"JOR": "Iordania",
	"JPN": "Japonia",
	"KAZ": "Kazahstan",
	"KEN": "Kenya",
	"KGZ": "Kargazstan",
	"KHM": "Cambodgia",
	"KIR": "Kiribati",
	"KNA": "Saint Kitts si Nevis",
	"KOR": "Coreea de Sud",
	"KWT": "Kuweit",
	"LAO": "Laos",
	"LBN": "Liban",
	"LBR": "Liberia",
	"LBY": "Libia",
	"LCA": "Saint Lucia",
	"LIE": "Liechtenstein",
	"LKA": "Sri Lanka",
	"LSO": "Lesotho",
	"LTU": "Lituania",
	"LUX": "Luxemburg",
	"LVA": "Letonia",
	"MAF": "Saint-Martin",
	"MAR": "Maroc",
	"MCO": "Monaco",
	"MDA": "Moldova",
	"MDG": "Madagascar",
	"MDV": "Maldive",
	"MEX": "Mexic",
	"MHL": "Insulele Marshall",
	"MKD": "Macedonia de Nord",
	"MLI": "Mali",
	"MLT": "Malta",
	"MMR": "Myanmar/Birmania",
	"MNE": "Muntenegru",
	"MNG": "Mongolia",
	"MOZ": "Mozambic",
	"MRT": "Mauritania",
	"MSR": "Montserrat",
	"MUS": "Mauritius",
	"MWI": "Malawi",
	"MYS": "Malaysia",
	"NAM": "Namibia",
	"NCL": "Noua Caledonie",
	"NER": "Niger",
	"NGA": "Nigeria",
	"NIC": "Nicaragua",
	"NLD": "Olanda",
	"NOR": "Norvegia",
	"NPL": "Nepal",
	"NRU": "Nauru",
	"NZL": "Noua Zeelanda",
	"OMN": "Oman",
	"PAK": "Pakistan",
	"PAN": "Panama",
	"PCN": "Insulele Pitcairn",
	"PER": "Peru",
	"PHL": "Filipine",
	"PLW": "Palau",
	"PNG": "Papua-Noua Guinee",
	"POL": "Polonia",
	"PRK": "Coreea de Nord",
	"PRT": "Portugalia",
	"PRY": "Paraguay",
	"PSE": "Palestinian Territory",
	"PYF": "Polinezia Franceza",
	"QAT": "Qatar",
	"ROU": "Romania",
	"RUS": "Rusia",
	"RWA": "Rwanda",
	"SAU": "Arabia Saudita",
	"SDN": "Sudan",
	"SEN": "Senegal",
	"SGP": "Singapore",
	"SHN": "Sfanta Elena, Ascension si Tristan da Cunha",
	"SLB": "Insulele Solomon",
	"SLE": "Sierra Leone",
	"SLV": "El Salvador",
	"SMR": "San Marino",
	"SOM": "Somalia",
	"SPM": "Saint-Pierre si Miquelon",
	"SRB": "Serbia",
	"SSD": "Sudanul de Sud",
	"STP": "Sao Tome si Principe",
	"SUR": "Suriname",
	"SVK": "Slovacia",
	"SVN": "Slovenia",
	"SWE": "Suedia",
	"SWZ": "Eswatini",
	"SXM": "Sint-Maarten",
	"SYC": "Seychelles",
	"SYR": "Siria",
	"TCA": "Insulele Turks si Caicos",
	"TCD": "Ciad",
	"TGO": "Togo",
	"THA": "Thailanda",
	"TJK": "Tadjikistan",
	"TKM": "Turkmenistan",
	"TLS": "Timorul de Est",
	"TON": "Tonga",
	"TTO": "Trinidad si Tobago",
	"TUN": "Tunisia",
	"TUR": "Turcia",
	"TUV": "Tuvalu",
	"TWN": "Taiwan",
	"TZA": "Tanzania",
	"UGA": "Uganda",
	"UKR": "Ucraina",
	"URY": "Uruguay",
	"USA": "Statele Unite",
	"UZB": "Uzbekistan",
	"VAT": "Sfantul Scaun/Vatican",
	"VCT": "Saint Vincent si Grenadinele",
	"VEN": "Venezuela",
	"VGB": "Insulele Virgine Britanice",
	"VNM": "Vietnam",
	"VUT": "Vanuatu",
	"WLF": "Wallis si Futuna",
	"WSM": "Samoa",
	"YEM": "Yemen",
	"ZAF": "Africa de Sud",
	"ZMB": "Zambia",
	"ZWE": "Zimbabwe",
	"GBR": "Regatul Unit (UK)",
	"GLP": "Guadelupa",
	"ASM": "Samoa Americana",
	"BES": "Bonaire, Saint Eustatius and Saba",
	"CXR": "Insula Christmas",
	"MAC": "Macao",
	"IOT": "Teritoriul Britanic din Oceanul Indian",
	"NIU": "Niue",
	"HKG": "Hong Kong",
	"UMI": "Insulele Minore Indepartate ale Statelor Unite",
	"GUF": "Guyana Franceza",
	"MTQ": "Martinica",
	"NFK": "Insula Norfolk",
	"PRI": "Puerto Rico",
	"ALA": "Insulele Aland",
	"GUM": "Guam",
	"MYT": "Mayotte",
	"ATA": "Antarctica",
	"SGS": "Georgia de Sud si Insulele Sandwich de Sud",
	"REU": "Reunion",
	"HMD": "Insula Heard si Insulele McDonald",
	"TKL": "Tokelau",
	"BVT": "Insula Bouvet",
	"SJM": "Svalbard si Jan Mayen",
	"CCK": "Insulele Cocos (Keeling)",
	"VIR": "Insulele Virgine Americane",
	"MNP": "Insulele Mariane de Nord",
}

const ROUVatPerc = 19_00

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

func isUeCountry(countryCode string) bool {
	_, ok := euCountries[strings.ToUpper(countryCode)]
	return ok
}

func GetEuVatPercentage(countryCode string) *int64 {
	vat, ok := euCountriesVat[strings.ToUpper(countryCode)]
	if !ok {
		return nil
	}
	return &vat
}

func GetRoNameForISOCode(isoCode string) string {
	name, ok := countriesName[strings.ToUpper(isoCode)]
	if !ok {
		return ""
	}
	return name
}
