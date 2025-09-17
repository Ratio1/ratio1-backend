package service

import (
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
)

func Test_monthlyPoaiInvoiceService(t *testing.T) {
	config.Config.Mail = config.MailConfig{
		ApiUrl:    "",
		ApiKey:    "",
		FromEmail: "",
	}
	config.Config.FreeCurrencyApiKey = ""
	config.Config.Database = config.DatabaseConfig{
		User:         "",
		Password:     "",
		Host:         "",
		Port:         0,
		DbName:       "",
		MaxOpenConns: 100,
		MaxIdleConns: 100,
		SslMode:      "disable",
	}
	storage.Connect()
	MonthlyPoaiInvoiceReport()
}
