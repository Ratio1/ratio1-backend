package service

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/templates"
)

func FillInvoiceDraftTemplate(invoice model.InvoiceDraft, allocations []model.Allocation) ([]byte, error) {
	vm := buildInvoiceView(invoice, allocations)

	tmpl, err := templates.GetInvoiceDraftTemplate()
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, vm); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return htmlBuf.Bytes(), nil
}

func FillInvoiceDraftTemplateJSON(invoice model.InvoiceDraft, allocations []model.Allocation) (*invoiceVM, error) {
	vm := buildInvoiceView(invoice, allocations)
	return &vm, nil

}

// ------------------ View model & helpers ------------------

type allocationRow struct {
	JobID              string
	JobName            string
	JobType            string
	ProjectName        string
	AllocationCreation string
	NodeAddress        string
	UsdcPaid           string
}

type extraLineVM struct {
	Label       string
	AmountUSDC  string
	AmountLocal string
}

type invoiceVM struct {
	Title         string
	Date          string
	InvoiceSeries string
	InvoiceNumber int

	SellerLines  []string
	BuyerLines   []string
	SellerWallet string
	BuyerWallet  string

	Allocations []allocationRow
	NetBase     string
	VatPerc     string
	VatAmount   string

	LocalCurrency  string
	TotalUSDC      string
	TotalLocal     string
	NetBaseLocal   string
	VatAmountLocal string

	ExtraLines []extraLineVM

	JobCount int
	Notes    string
	Status   string
}

func buildInvoiceView(invoice model.InvoiceDraft, allocations []model.Allocation) invoiceVM {
	// Seller/Buyer lines
	from := &invoice.UserProfile
	to := &invoice.CspProfile
	fromLines := formatUserInfo(from)
	toLines := formatUserInfo(to)

	// Economic summary
	extras, _ := invoice.GetExtraTaxes()
	sumFixedLocalCurrency, sumExtraPerc := 0.0, 0.0
	for _, e := range extras {
		switch e.TaxType {
		case model.Fixed:
			sumFixedLocalCurrency += e.Value
		case model.Percentage:
			sumExtraPerc += e.Value
		}
	}

	sumFixed := 0.0
	if sumFixedLocalCurrency > 0 {
		sumFixed = sumFixedLocalCurrency / invoice.LocalCurrencyExchangeRatio
	}

	den := 1.0 + (invoice.VatApplied / 100.0) + (sumExtraPerc / 100.0)
	netBase := (invoice.TotalUsdcAmount - sumFixed) / den
	if netBase < 0 {
		netBase = 0
	}
	vatAmount := netBase * (invoice.VatApplied / 100.0)

	var extraVM []extraLineVM
	for _, e := range extras {
		switch e.TaxType {
		case model.Fixed:
			extraVM = append(extraVM, extraLineVM{
				Label:       e.Description,
				AmountUSDC:  fmt.Sprintf("%.2f", e.Value/invoice.LocalCurrencyExchangeRatio),
				AmountLocal: fmt.Sprintf("%.2f", e.Value),
			})
		case model.Percentage:
			extraVM = append(extraVM, extraLineVM{
				Label:       fmt.Sprintf("%s (%.2f%%)", e.Description, e.Value),
				AmountUSDC:  fmt.Sprintf("%.2f", netBase*(e.Value/100.0)),
				AmountLocal: fmt.Sprintf("%.2f", netBase*(e.Value/100.0)*invoice.LocalCurrencyExchangeRatio),
			})
		}
	}

	// Alloc rows
	/*Order allocations based on creation timestamp*/
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].AllocationCreation.Before(allocations[j].AllocationCreation)
	})

	var allocRows []allocationRow
	for _, a := range allocations {
		allocRows = append(allocRows, allocationRow{
			AllocationCreation: a.AllocationCreation.Format("2006-01-02"),
			JobID:              a.JobId,
			JobName:            a.JobName,
			JobType:            a.JobType.GetName(),
			ProjectName:        a.ProjectName,
			NodeAddress:        a.NodeAddress[:5] + "..." + a.NodeAddress[len(a.NodeAddress)-5:],
			UsdcPaid:           GetAmountAsFloatString(a.GetUsdcAmountPayed(), model.UsdcDecimals),
		})
	}

	title := "Invoice Draft"
	if !invoice.UserProfile.IsCompany {
		title = "Consumption Report"
	}
	//draft fields filling
	vm := invoiceVM{
		//header
		Title:         title,
		Date:          invoice.CreationTimestamp.Format("2006-01-02"),
		InvoiceSeries: invoice.InvoiceSeries,
		InvoiceNumber: invoice.InvoiceNumber,
		SellerLines:   fromLines,
		BuyerLines:    toLines,
		SellerWallet:  from.BlockchainAddress,
		BuyerWallet:   to.BlockchainAddress,

		//economic summary
		TotalUSDC:      fmt.Sprintf("%.2f", invoice.TotalUsdcAmount),
		NetBase:        fmt.Sprintf("%.2f", netBase),
		NetBaseLocal:   fmt.Sprintf("%.2f", netBase*invoice.LocalCurrencyExchangeRatio),
		VatPerc:        fmt.Sprintf("%.2f", invoice.VatApplied),
		VatAmount:      fmt.Sprintf("%.2f", vatAmount),
		ExtraLines:     extraVM,
		JobCount:       len(allocations),
		Notes:          safe(invoice.ExtraText),
		LocalCurrency:  invoice.LocalCurrency,
		TotalLocal:     fmt.Sprintf("%.2f", invoice.TotalUsdcAmount*invoice.LocalCurrencyExchangeRatio),
		VatAmountLocal: fmt.Sprintf("%.2f", vatAmount*invoice.LocalCurrencyExchangeRatio),

		Status: "Draft (non-fiscal document)",

		//allocation report
		Allocations: allocRows,
	}
	return vm
}

// ---- Helpers ----

func formatUserInfo(u *model.UserInfo) []string {
	if u == nil {
		return []string{""}
	}
	lines := []string{}
	if u.IsCompany {
		lines = append(lines, nonEmptyOr("-", safe(u.CompanyName)))
	} else {
		fullName := fmt.Sprintf("%s %s", nonEmptyOr("", safe(u.Name)), nonEmptyOr("", safe(u.Surname)))
		if fullName == " " {
			fullName = "-"
		}
		lines = append(lines, fullName)
	}
	if u.Address != "" {
		lines = append(lines, u.Address)
	}
	cityline := joinNonEmpty(", ", []string{u.City, u.State, u.Country})
	if cityline != "" {
		lines = append(lines, cityline)
	}
	if u.IdentificationCode != "" {
		lines = append(lines, u.IdentificationCode)
	}
	if u.Email != "" {
		lines = append(lines, "Email: "+u.Email)
	}
	return lines
}

func safe(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmptyOr(def, val string) string {
	if val == "" {
		return def
	}
	return val
}

func joinNonEmpty(sep string, parts []string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += sep
		}
		out += p
	}
	return out
}
