package service

import (
	"os"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_generateInvoiceDraft(t *testing.T) {
	// --- Test profiles ---
	firstName := "Mario"
	lastName := "Rossi"
	company := "ACME LTD"

	seller := model.UserInfo{
		Email:              "billing@acme.example",
		Name:               &firstName,
		Surname:            &lastName,
		CompanyName:        &company,
		IdentificationCode: "IT12345678901",
		Address:            "1 Roma Street",
		State:              "MI",
		City:               "Milan",
		Country:            "Italy",
		IsCompany:          true,
		BlockchainAddress:  "0xCSPACME00000000000000000000000000000000",
	}

	buyer := model.UserInfo{
		Email:              "mario.rossi@example.com",
		Name:               &firstName,
		Surname:            &lastName,
		IdentificationCode: "IT98765432109",
		Address:            "10 Milano Avenue",
		State:              "TO",
		City:               "Turin",
		Country:            "Italy",
		IsCompany:          false,
		BlockchainAddress:  "0xUSER000000000000000000000000000000000000",
	}

	// --- Invoice (draft) ---
	invoiceID := uuid.New()
	extra := "On-chain services as per payment details."

	// Extra taxes as JSON (to test unmarshal):
	// TaxType: Fixed=0, Percentage=1
	// - Network fee: fixed 10.00
	// - Service surcharge: 5% (percentage)
	extraTaxesJSON := `[
		{ "Description": "Network fee", "TaxType": 0, "Value": 10.0 },
		{ "Description": "Service surcharge", "TaxType": 1, "Value": 5.0 }
	]`

	invoice := model.InvoiceDraft{
		DraftId:           invoiceID,
		CreationTimestamp: time.Now(),
		UserAddress:       buyer.BlockchainAddress,
		CspOwner:          "0xOWNERACME000000000000000000000000000000",
		CspProfile:        seller,
		UserProfile:       buyer,
		InvoiceSeries:     "ACME-2024",
		InvoiceNumber:     2,
		TotalUsdcAmount:   150.00, // GROSS amount (includes VAT & extras)
		VatApplied:        22.0,   // VAT 22%
		ExtraText:         &extra,
		ExtraTaxes:        &extraTaxesJSON, // JSON string to exercise GetExtraTaxes()
	}

	// --- Allocations (payments) ---
	allocations := []model.Allocation{
		{
			BlockNumber:     1234567,
			TxHash:          "0xabc1111111111111111111111111111111111111111111111111111111111111",
			JobId:           "job-001",
			NodeAddress:     "0xNODE000000000000000000000000000000000001",
			UserAddress:     buyer.BlockchainAddress,
			CspProfile:      seller,
			UserProfile:     buyer,
			CspAddress:      seller.BlockchainAddress,
			CspOwner:        invoice.CspOwner,
			JobName:         "Compute-Optimized VM",
			JobType:         1,
			ProjectName:     "Project Alpha",
			UsdcAmountPayed: "100",
			DraftId:         &invoiceID,
		},
		{
			BlockNumber:     1234570,
			TxHash:          "0xdef2222222222222222222222222222222222222222222222222222222222222",
			JobId:           "job-002",
			NodeAddress:     "0xNODE000000000000000000000000000000000002",
			UserAddress:     buyer.BlockchainAddress,
			CspProfile:      seller,
			UserProfile:     buyer,
			CspAddress:      seller.BlockchainAddress,
			CspOwner:        invoice.CspOwner,
			JobName:         "Storage-Optimized VM",
			JobType:         2,
			ProjectName:     "Project Beta",
			UsdcAmountPayed: "50",
			DraftId:         &invoiceID,
		},
	}

	// --- Generate .doc ---
	file, err := FillInvoiceDraftTemplate(invoice, allocations)
	require.Nil(t, err)

	if err := os.WriteFile("invoice_draft.doc", file, 0644); err != nil {
		require.Nil(t, err)
	}
}
