package service

//TODO redo all
import (
	"bytes"
	"fmt"
	"sort"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/templates"
)

func GenerateInvoiceDOC(invoice model.InvoiceDraft, allocations []model.Allocation) ([]byte, error) {
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
	/*Order allocations based on creation timestamp*/
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].AllocationCreation.Before(allocations[j].AllocationCreation)
	})

	// Seller/Buyer lines
	from := &invoice.UserProfile
	to := &invoice.CspProfile
	fromLines := formatUserInfo(from)
	toLines := formatUserInfo(to)

	// Alloc rows
	var allocRows []allocationRow
	for _, a := range allocations {
		allocRows = append(allocRows, allocationRow{

			AllocationCreation: a.AllocationCreation.Format("2006-01-02"),
			JobID:              truncate(a.JobId, 26),
			JobName:            a.JobName,
			JobType:            a.JobType.GetName(),
			ProjectName:        a.ProjectName,
			NodeAddress:        a.NodeAddress[:5] + "..." + a.NodeAddress[len(a.NodeAddress)-5:],
			UsdcPaid:           GetAmountAsFloatString(a.GetUsdcAmountPayed(), model.UsdcDecimals),
		})
	}

	// Economic summary
	vatPerc := invoice.VatApplied

	extras, _ := invoice.GetExtraTaxes()
	var sumFixed, sumExtraPerc float64
	for _, e := range extras {
		switch e.TaxType {
		case model.Fixed:
			sumFixed += e.Value
		case model.Percentage:
			sumExtraPerc += e.Value
		}
	}
	den := 1.0 + (vatPerc / 100.0) + (sumExtraPerc / 100.0)
	netBase := 0.0
	if den > 0 {
		netBase = (invoice.TotalUsdcAmount - sumFixed) / den
	}
	if netBase < 0 {
		netBase = 0
	}
	vatAmount := netBase * (vatPerc / 100.0)

	var extraVM []extraLineVM
	for _, e := range extras {
		switch e.TaxType {
		case model.Fixed:
			extraVM = append(extraVM, extraLineVM{
				Label:       e.Description,
				AmountUSDC:  fmt.Sprintf("%.2f", e.Value),
				AmountLocal: fmt.Sprintf("%.2f", e.Value*invoice.LocalCurrencyExchangeRatio),
			})
		case model.Percentage:
			extraVM = append(extraVM, extraLineVM{
				Label:       fmt.Sprintf("%s (%.2f%%)", e.Description, e.Value),
				AmountUSDC:  fmt.Sprintf("%.2f", netBase*(e.Value/100.0)),
				AmountLocal: fmt.Sprintf("%.2f", netBase*(e.Value/100.0)*invoice.LocalCurrencyExchangeRatio),
			})
		}
	}

	// Status
	status := "Draft (non-fiscal document)"

	vm := invoiceVM{
		Title:         "Invoice Draft",
		Date:          invoice.CreationTimestamp.Format("2006-01-02"),
		InvoiceSeries: invoice.InvoiceSeries,
		InvoiceNumber: invoice.InvoiceNumber,

		SellerLines:  fromLines,
		BuyerLines:   toLines,
		SellerWallet: walletOrEmpty(from),
		BuyerWallet:  walletOrEmpty(to),

		Allocations: allocRows,

		TotalUSDC:    fmt.Sprintf("%.2f", invoice.TotalUsdcAmount),
		NetBase:      fmt.Sprintf("%.2f", netBase),
		NetBaseLocal: fmt.Sprintf("%.2f", netBase*invoice.LocalCurrencyExchangeRatio),
		VatPerc:      fmt.Sprintf("%.2f", vatPerc),
		VatAmount:    fmt.Sprintf("%.2f", vatAmount),

		ExtraLines: extraVM,

		JobCount: len(allocations),
		Notes:    stringOrEmpty(invoice.ExtraText),
		Status:   status,

		LocalCurrency:  invoice.LocalCurrency,
		TotalLocal:     fmt.Sprintf("%.2f", invoice.TotalUsdcAmount*invoice.LocalCurrencyExchangeRatio),
		VatAmountLocal: fmt.Sprintf("%.2f", vatAmount*invoice.LocalCurrencyExchangeRatio),
	}
	return vm
}

func walletOrEmpty(u *model.UserInfo) string {
	if u != nil && u.BlockchainAddress != "" {
		return truncate(u.BlockchainAddress, 50)
	}
	return ""
}

// ---- Helpers ----

func stringOrEmpty(ps *string) string {
	if ps == nil {
		return ""
	}
	return *ps
}

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
	if u.CompanyName != nil && *u.CompanyName != "" && !u.IsCompany {
		lines = append(lines, *u.CompanyName)
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

func truncate(s string, max int) string {
	if max <= 3 || len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
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
