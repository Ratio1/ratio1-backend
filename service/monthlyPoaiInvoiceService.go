package service

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
		invoice, err := createMonthlyPoaiDraft(userAddress, cspOwner, allocations, currencyMap)
		if err != nil {
			fmt.Println("error while saving invoice: " + err.Error())
			continue
		}
		drafts = append(drafts, *invoice)
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

func createMonthlyPoaiDraft(userAddress, cspOwner string, allocations []model.Allocation, currencyMap map[string]float64) (*model.InvoiceDraft, error) {
	if len(allocations) == 0 {
		return nil, fmt.Errorf("no allocations found for pair %s-%s", userAddress, cspOwner)
	}

	invoice := model.InvoiceDraft{
		DraftId:           uuid.New(),
		UserAddress:       userAddress,
		CspOwner:          cspOwner,
		CreationTimestamp: time.Now(),
		UserProfile:       allocations[0].UserProfile,
		CspProfile:        allocations[0].CspProfile,
	}

	err := storage.WithTransaction(func(tx *gorm.DB) error {
		preference, preferenceExists, err := loadDraftPreference(tx, userAddress, &invoice)
		if err != nil {
			return err
		}

		totalUsdcAmount := big.NewInt(0)
		for _, alloc := range allocations {
			totalUsdcAmount.Add(totalUsdcAmount, alloc.GetUsdcAmountPayed())
			alloc.DraftId = &invoice.DraftId
			if err := storage.UpdateAllocation(tx, &alloc); err != nil {
				return fmt.Errorf("error while updating allocation: %w", err)
			}
		}
		invoice.TotalUsdcAmount += GetAmountAsFloat(totalUsdcAmount, model.UsdcDecimals)

		if userAddress != cspOwner {
			preference.NextNumber += 1
			if preferenceExists {
				if err := storage.UpdatePreference(tx, preference); err != nil {
					return fmt.Errorf("error while updating preference: %w", err)
				}
			} else if err := storage.CreatePreference(tx, preference); err != nil {
				return fmt.Errorf("error while creating preference: %w", err)
			}
		} else {
			invoice.InvoiceNumber = 0
			invoice.InvoiceSeries = ""
		}

		if v, ok := currencyMap[invoice.LocalCurrency]; ok {
			invoice.LocalCurrencyExchangeRatio = v
		}
		if err := storage.CreateInvoiceDraft(tx, &invoice); err != nil {
			return fmt.Errorf("error while saving invoice: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &invoice, nil
}

func loadDraftPreference(tx *gorm.DB, userAddress string, invoice *model.InvoiceDraft) (*model.Preference, bool, error) {
	preference, err := storage.GetPreferenceByAddressForUpdate(tx, userAddress)
	if err != nil {
		return nil, false, fmt.Errorf("error while retrieving user preference: %w", err)
	}
	preferenceExists := preference != nil
	if !preferenceExists {
		preference = &model.Preference{
			UserAddress:   userAddress,
			NextNumber:    1,
			InvoiceSeries: "NODE",
			CountryVat:    0,
			UeVat:         0,
			ExtraUeVat:    0,
			LocalCurrency: "USD",
		}
	}
	applyDraftPreference(invoice, preference)

	return preference, preferenceExists, nil
}

func applyDraftPreference(invoice *model.InvoiceDraft, preference *model.Preference) {
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
}
