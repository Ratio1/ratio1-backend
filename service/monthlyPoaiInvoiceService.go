package service

import (
	"fmt"
	"strings"
	"time"

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
				InvoiceSeries: "-NODE",
				CountryVat:    0,
				UeVat:         0,
				ExtraUeVat:    0,
			}
			invoice.VatApplied = preference.ExtraUeVat
			invoice.InvoiceNumber = preference.NextNumber
			invoice.InvoiceSeries = preference.InvoiceSeries
		}

		for _, alloc := range allocations {
			invoice.TotalUsdcAmount += GetAmountAsFloat(alloc.GetUsdcAmountPayed(), model.UsdcDecimals)
			alloc.DraftId = &invoice.DraftId
			err = storage.UpdateAllocation(&alloc) //TODO create more stable system with rollback for all invoices
			if err != nil {
				fmt.Println("error while updating allocation: " + err.Error())
				return
			}
		}

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

	allCSP := make(map[string]bool) //map[email]true to have unique emails
	allNodeOwner := make(map[string]bool)
	for _, invoice := range drafts {
		if invoice.UserAddress != invoice.CspOwner { // I should not receive emails if i worked on my nodes
			allNodeOwner[invoice.UserProfile.Email] = true
			allCSP[invoice.CspProfile.Email] = true
		}
	}

	//send unique email for csp and node owner ( even if they have more than 1 invoice)
	for k := range allNodeOwner {
		_ = SendNodeOwnerDraftEmail(k) //! doesn't check error
	}

	for k := range allCSP {
		_ = SendCspDraftEmail(k) //! doesn't check error
	}
}

func formKey(address1, address2 string) string {
	return address1 + "-" + address2
}

func splitKey(key string) (string, string) {
	parts := strings.Split(key, "-")
	return parts[0], parts[1]
}
