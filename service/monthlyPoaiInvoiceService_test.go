package service

import (
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/stretchr/testify/require"
)

func TestDraftNotificationEmailsUsesConfirmedPrimaryAndSecondary(t *testing.T) {
	previousGetAccount := getAccountByAddressFn
	previousGetNotificationEmail := getNotificationEmailFn
	defer func() {
		getAccountByAddressFn = previousGetAccount
		getNotificationEmailFn = previousGetNotificationEmail
	}()

	primaryEmail := "Owner@Example.com"
	secondaryEmail := " Ops@Example.com "
	getAccountByAddressFn = func(address string) (*model.Account, bool, error) {
		require.Equal(t, "0xowner", address)
		return &model.Account{
			Address:        address,
			Email:          &primaryEmail,
			EmailConfirmed: true,
		}, true, nil
	}
	getNotificationEmailFn = func(address string) (*model.AccountNotificationEmail, bool, error) {
		require.Equal(t, "0xowner", address)
		return &model.AccountNotificationEmail{
			AccountAddress: address,
			Email:          &secondaryEmail,
			EmailConfirmed: true,
		}, true, nil
	}

	require.Equal(t, []string{"owner@example.com", "ops@example.com"}, draftNotificationEmails("0xowner", "fallback@example.com"))
}

func TestDraftInvoiceAttachmentName(t *testing.T) {
	supplier := "Acme Nodes SRL"
	beneficiary := "Ratio/Cloud:EU"
	draft := model.InvoiceDraft{
		CreationTimestamp: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		InvoiceNumber:     42,
		InvoiceSeries:     "NODE",
		UserProfile: model.UserInfo{
			CompanyName: &supplier,
			IsCompany:   true,
		},
		CspProfile: model.UserInfo{
			CompanyName: &beneficiary,
			IsCompany:   true,
		},
	}

	require.Equal(t, "202607_Acme-Nodes-SRL_Ratio-Cloud-EU_42-NODE.doc", draftInvoiceAttachmentName(draft, "doc"))
}

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
