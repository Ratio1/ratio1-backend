package service

import (
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
)

func MonthlyPoaiInvoiceReport() {
	/* Get all Allocations not invoiced*/
	now := time.Now().UTC()
	unclaimedAllocations, err := storage.GetMonthlyUnclaimedAllocations(now)
	if err != nil {
		fmt.Println("Error retrieving unclaimed allocations: " + err.Error())
		return
	}

	if len(unclaimedAllocations) == 0 {
		fmt.Println("Drafts already done")
		return
	}

	currencyMap, err := GetFreeCurrencyValues() //map[USD,EUR...]ratio always based 1 usd -> value
	if err != nil {
		fmt.Println("could not fetch currency map: ", err.Error())
		return
	}

	/*Get all unique nodeOwner - csp pair*/
	reports := make(map[string][]model.Allocation)
	for _, alloc := range unclaimedAllocations {
		key := formKey(alloc.UserAddress, alloc.CspOwner)
		reports[key] = append(reports[key], alloc)
	}

	/* Generate invoices for each unique pair of csp and node owner*/
	var drafts []model.InvoiceDraft
	for k, allocations := range reports {
		userAddress, cspOwner := splitKey(k)
		invoice := model.InvoiceDraft{
			DraftId:           uuid.New(),
			UserAddress:       userAddress,
			CspOwner:          cspOwner,
			CreationTimestamp: time.Now(),
			UserProfile:       allocations[0].UserProfile,
			CspProfile:        allocations[0].CspProfile,
		}

		preference, err := storage.GetPreferenceByAddress(userAddress)
		if err != nil {
			fmt.Println("error while retrieving user preference: " + err.Error())
			continue
		} else if preference != nil {
			if invoice.CspProfile.Country == invoice.UserProfile.Country {
				invoice.VatApplied = preference.CountryVat
			} else if isUeCountry(invoice.UserProfile.Country) {
				if isUeCountry(invoice.CspProfile.Country) {
					invoice.VatApplied = preference.UeVat
				} else {
					invoice.VatApplied = preference.ExtraUeVat
				}
			} else {
				invoice.VatApplied = preference.ExtraUeVat
			}
			invoice.InvoiceNumber = preference.NextNumber
			invoice.InvoiceSeries = preference.InvoiceSeries
			invoice.ExtraTaxes = preference.ExtraTaxes
			invoice.ExtraText = preference.ExtraText
			invoice.LocalCurrency = preference.LocalCurrency
		} else {
			preference = &model.Preference{
				UserAddress:   userAddress,
				NextNumber:    1,
				InvoiceSeries: "NODE",
				CountryVat:    0,
				UeVat:         0,
				ExtraUeVat:    0,
				LocalCurrency: "USD",
			}
			invoice.VatApplied = preference.ExtraUeVat
			invoice.InvoiceNumber = preference.NextNumber
			invoice.InvoiceSeries = preference.InvoiceSeries
		}

		totalUsdcAmount := big.NewInt(0)
		for _, alloc := range allocations {
			totalUsdcAmount.Add(totalUsdcAmount, alloc.GetUsdcAmountPayed())
			alloc.DraftId = &invoice.DraftId
			err = storage.UpdateAllocation(&alloc) //TODO create more stable system with rollback for all invoices
			if err != nil {
				fmt.Println("error while updating allocation: " + err.Error())
				return
			}
		}
		invoice.TotalUsdcAmount += GetAmountAsFloat(totalUsdcAmount, model.UsdcDecimals)

		if userAddress != cspOwner {
			preference.NextNumber += 1
			err = storage.UpdatePreference(preference)
			if err != nil {
				fmt.Println("error while updating preference: " + err.Error())
				return
			}
		} else {
			invoice.InvoiceNumber = 0
			invoice.InvoiceSeries = ""
		}

		if v, ok := currencyMap[invoice.LocalCurrency]; ok {
			invoice.LocalCurrencyExchangeRatio = v
		}
		err = storage.CreateInvoiceDraft(&invoice)
		if err != nil {
			fmt.Println("error while saving invoice: " + err.Error())
			continue
		}
		drafts = append(drafts, invoice)
	}

	cspEmails := make(map[string][]string)
	nodeOwnerDrafts := make(map[string][]model.InvoiceDraft)
	for _, invoice := range drafts {
		if invoice.UserAddress != invoice.CspOwner { // I should not receive emails if i worked on my nodes
			nodeOwnerDrafts[invoice.UserAddress] = append(nodeOwnerDrafts[invoice.UserAddress], invoice)
			if _, found := cspEmails[invoice.CspOwner]; !found {
				cspEmails[invoice.CspOwner] = draftNotificationEmails(invoice.CspOwner, invoice.CspProfile.Email)
			}
		}
	}

	//send unique email for csp and node owner ( even if they have more than 1 invoice)
	for address, invoices := range nodeOwnerDrafts {
		attachments, err := draftInvoiceAttachments(invoices)
		if err != nil {
			fmt.Println("error while generating draft invoice attachments: " + err.Error())
			continue
		}
		for _, email := range draftNotificationEmails(address, invoices[0].UserProfile.Email) {
			_ = SendNodeOwnerDraftEmail(email, attachments...) //! doesn't check error
		}
	}

	for _, emails := range cspEmails {
		for _, email := range emails {
			_ = SendCspDraftEmail(email) //! doesn't check error
		}
	}
}

func draftNotificationEmails(address, fallbackEmail string) []string {
	emails, err := getConfirmedAccountEmails(address)
	if err != nil {
		fmt.Println("error while retrieving draft notification emails: " + err.Error())
		return nil
	}
	if len(emails) > 0 {
		return emails
	}

	email := TrimWhitespacesAndToLower(fallbackEmail)
	if email == "" {
		return nil
	}
	return []string{email}
}

func draftInvoiceAttachments(drafts []model.InvoiceDraft) ([]EmailAttachment, error) {
	attachments := make([]EmailAttachment, 0, len(drafts))
	for _, draft := range drafts {
		allocations, err := storage.GetAllocationsByDraftId(draft.DraftId.String())
		if err != nil {
			return nil, err
		}
		content, err := FillInvoiceDraftTemplate(draft, allocations)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, newEmailAttachment(draftInvoiceAttachmentName(draft, ".doc"), "application/msword", content))
	}
	return attachments, nil
}

func draftInvoiceAttachmentName(draft model.InvoiceDraft, ext string) string {
	if ext == "" {
		ext = ".doc"
	}
	if ext[0] != '.' {
		ext = "." + ext
	}

	supplier, ok := draft.UserProfile.GetNameAsString()
	if !ok {
		supplier = draft.UserAddress
	}
	beneficiary, ok := draft.CspProfile.GetNameAsString()
	if !ok {
		beneficiary = draft.CspOwner
	}
	invoiceNumber := fmt.Sprintf("%d", draft.InvoiceNumber)
	if strings.TrimSpace(draft.InvoiceSeries) != "" {
		invoiceNumber += "-" + draft.InvoiceSeries
	}

	return fmt.Sprintf("%s_%s_%s_%s%s",
		draft.CreationTimestamp.Format("200601"),
		safeFileNamePart(supplier),
		safeFileNamePart(beneficiary),
		safeFileNamePart(invoiceNumber),
		ext,
	)
}

func safeFileNamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}

	var out strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}

	name := strings.Trim(out.String(), "-")
	if name == "" {
		return "unknown"
	}
	return name
}

func formKey(address1, address2 string) string {
	return address1 + "-" + address2
}

func splitKey(key string) (string, string) {
	parts := strings.Split(key, "-")
	return parts[0], parts[1]
}
