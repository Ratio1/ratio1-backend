package model

const InvoiceStatusPending = "pending"
const InvoiceStatusPaid = "paid"

const (
	InvoiceCif           = "50252500"
	InvoiceSeriesName    = "NAE"
	InvoiceROUSeriesName = "NAERO"
	ROU_ID               = "ROU"
)

type InvoiceClient struct {
	Uuid               *string `gorm:"primarykey;"`
	Name               *string `gorm:"default:null" json:"name"`
	Surname            *string `gorm:"default:null" json:"surname"`
	CompanyName        *string `gorm:"default:null" json:"companyName"`
	UserEmail          *string `gorm:"not null"`
	IdentificationCode string  `gorm:"not null" json:"identificationCode"`
	Address            string  `json:"address"`
	State              string  `json:"state"`
	City               string  `json:"city"`
	Country            string  `json:"country"`
	IsCompany          bool    `json:"isCompany"`
	Status             *string `gorm:"not null" `
	InvoiceUrl         *string `gorm:"default:null"`
	InvoiceNumber      *string `gorm:"default:null"`
	TxHash             *string `gorm:"default:null"`
	BlockNumber        *int64  `gorm:"default:null"`
	ReverseCharge      bool    `gorm:"default:false"`
	IsUe               bool    `gorm:"default:true"`
	NumLicenses        *int    `gorm:"default:null"`
	UnitUsdPrice       *int    `gorm:"default:null"`
}

type AuthRequest struct {
	AccessToken string      `json:"access_token"`
	ExpiresIn   interface{} `json:"expires_in"`
	TokenType   string      `json:"token_type"`
	Scope       string      `json:"scope"`
	RequestTime interface{} `json:"request_time"`
}

type InvoiceRequest struct {
	CIF        string             `json:"cif"`
	SeriesName string             `json:"seriesName"`
	Number     int                `json:"number"`
	Currency   string             `json:"currency,omitempty"`
	Language   string             `json:"language"`
	Client     OblioInvoiceClient `json:"client"`
	Product    []InvoiceProduct   `json:"products"`
	SendEmail  int                `json:"sendEmail,omitempty"`
	Mentions   string             `json:"mentions,omitempty"`
	Collect    InvoiceCollect     `json:"collect,omitempty"`
}

type OblioInvoiceClient struct {
	CIF     string `json:"cif"`
	Name    string `json:"name"`
	Address string `json:"address"`
	State   string `json:"state"`
	City    string `json:"city"`
	Country string `json:"country"`
	Email   string `json:"email"`
}
type InvoiceProduct struct {
	Name  string `json:"name"`
	Price int64  `json:"price,omitempty"`

	MeasuringUnit string  `json:"measuringUnit,omitempty"`
	VatName       string  `json:"vatName,omitempty"`
	VatPercentage float64 `json:"vatPercentage"`
	VatIncluded   int     `json:"vatIncluded,omitempty"`

	Quantity int64  `json:"quantity,omitempty"`
	Currency string `json:"currency,omitempty"`
}

type InvoiceCollect struct {
	Type           string `json:"type"`
	DocumentNumber string `json:"documentNumber,omitempty"`
}

type OblioInvoiceResponse struct {
	Status        int64  `json:"status"`
	StatusMessage string `json:"statusMessage"`
	Data          Data   `json:"data"`
}

type Data struct {
	SeriesName string `json:"seriesName"`
	Number     string `json:"number"`
	Link       string `json:"link"`
}

type InvoiceWebHookRequest struct {
	Address      string  `json:"address"`
	InvoiceID    string  `json:"invoiceID"`
	NumLicenses  int     `json:"numLicenses"`
	UnitUsdPrice float64 `json:"unitUsdPrice"`
	TokenPaid    string  `json:"tokenPaid"`
}

type Event struct {
	Address      string  `json:"address"`
	InvoiceID    string  `json:"invoiceID"`
	NumLicenses  int     `json:"numLicenses"`
	UnitUsdPrice int     `json:"unitUsdPrice"`
	TokenPaid    float64 `json:"tokenPaid"`
	TxHash       string  `json:"txHash"`
	BlockNumber  int64   `json:"blockNumber"`
}
