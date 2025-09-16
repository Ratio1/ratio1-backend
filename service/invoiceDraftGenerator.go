package service

//TODO redo all
import (
	"bytes"
	"fmt"
	"math/big"

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
	JobID       string
	JobName     string
	JobType     string
	ProjectName string
	NodeAddress string
	UsdcPaid    string
}

type extraLineVM struct {
	Label  string
	Amount string
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

	TotalUSDC string
	NetBase   string
	VatPerc   string
	VatAmount string

	ExtraLines []extraLineVM

	Gross    string
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

	// Alloc rows + subtotal USDC
	totalUSDC := big.NewInt(0)
	var allocRows []allocationRow
	for _, a := range allocations {
		allocRows = append(allocRows, allocationRow{
			JobID:       truncate(a.JobId, 26),
			JobName:     a.JobName,
			JobType:     a.JobType.GetName(),
			ProjectName: a.ProjectName,
			NodeAddress: a.NodeAddress,
			UsdcPaid:    a.UsdcAmountPayed,
		})
		totalUSDC.Add(totalUSDC, a.GetUsdcAmountPayed())
	}

	// Economic summary
	gross := invoice.TotalUsdcAmount
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
		netBase = (gross - sumFixed) / den
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
				Label:  e.Description,
				Amount: fmt.Sprintf("%.2f", e.Value),
			})
		case model.Percentage:
			extraVM = append(extraVM, extraLineVM{
				Label:  fmt.Sprintf("%s (%.2f%%)", e.Description, e.Value),
				Amount: fmt.Sprintf("%.2f", netBase*(e.Value/100.0)),
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

		TotalUSDC: totalUSDC.String(),
		NetBase:   fmt.Sprintf("%.2f", netBase),
		VatPerc:   fmt.Sprintf("%.2f", vatPerc),
		VatAmount: fmt.Sprintf("%.2f", vatAmount),

		ExtraLines: extraVM,

		Gross:    fmt.Sprintf("%.2f", gross),
		JobCount: len(allocations),
		Notes:    stringOrEmpty(invoice.ExtraText),
		Status:   status,
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
