package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
)

func MonthlyPoaiIncoiceReport() {
	/* Get all Allocations not invoiced*/
	unclaimedAllocations, err := storage.GetUnClaimedAllocations() //TODO with data between start-end month
	if err != nil {
		fmt.Println("Error retrieving unclaimed allocations: " + err.Error())
		return
	}

	if len(unclaimedAllocations) == 0 {
		fmt.Println("Drafts already done")
		return
	}

	/*Get all unique nodeOwner - csp pair*/
	reports := make(map[string][]model.Allocation)
	for _, alloc := range unclaimedAllocations {
		key := formKey(alloc.UserAddress, alloc.CspOwner)
		reports[key] = append(reports[key], alloc)
	}

	/* Generate invoices for each unique pair of csp and node owner*/
	var invoices []model.InvoiceDraft
	for k, allocations := range reports {
		userAddress, cspOwner := splitKey(k)
		invoice := model.InvoiceDraft{
			DraftId:           uuid.New(),
			UserAddress:       userAddress,
			CspOwner:          cspOwner,
			CreationTimestamp: time.Now(),
		}
		invoice.UserProfile = allocations[0].UserProfile
		invoice.CspProfile = allocations[0].CspProfile

		preference, err := storage.GetPreferenceByAddress(userAddress)
		if err != nil {
			fmt.Println("error while retrieving user preference: " + err.Error())
			continue
		} else if preference != nil {
			if invoice.CspProfile.Country == invoice.UserProfile.Country {
				invoice.VatApplied = preference.Country
			} else if isUeCountry(invoice.UserProfile.Country) {
				if isUeCountry(invoice.CspProfile.Country) {
					invoice.VatApplied = preference.Ue
				} else {
					invoice.VatApplied = preference.ExtraUe
				}
			}
			invoice.InvoiceNumber = preference.NextNumber
			invoice.InvoiceSeries = preference.InvoiceSeries
			invoice.ExtraTaxes = preference.ExtraTaxes
			invoice.ExtraText = preference.ExtraText
		}

		for _, alloc := range allocations {
			invoice.TotalUsdcAmount += GetAmountAsFloat(alloc.GetUsdcAmountPayed(), model.UsdcDecimals)
			alloc.DraftId = &invoice.DraftId
			err = storage.UpdateAllocation(&alloc)
			if err != nil {
				fmt.Println("error while updating allocation: " + err.Error())
				return
			} else {
				invoices = append(invoices, invoice)
			}
		}

		if userAddress != cspOwner {
			preference.NextNumber += 1
			err = storage.UpdatePreference(preference)
			if err != nil {
				fmt.Println(errors.New("error while updating preference: " + err.Error()))
				return
			}
		} else {
			invoice.InvoiceNumber = 0
			invoice.InvoiceSeries = ""
		}

		//save draft to db
		err = storage.CreateInvoiceDraft(&invoice)
		if err != nil {
			fmt.Println(errors.New("error while saving invoice: " + err.Error()))
			return
		}
	}

	allCSP := make(map[string]bool) //map[email]true to have unique emails
	allNodeOwner := make(map[string]bool)
	for _, invoice := range invoices {
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
