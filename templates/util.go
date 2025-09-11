package templates

import (
	"html/template"
	"path/filepath"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
)

const (
	emailConfirmFile         = "email.confirm.html"
	emailKycRejectedFile     = "email.final.rejected.html"
	emailStepRejectedFile    = "email.step.rejected.html"
	emailBlacklistedFile     = "email.blacklisted.html"
	emailKycConfirmedFile    = "email.kyc.confirmed.html"
	emailAccountResettedFile = "email.account.resetted.html"

	invoiceDraftFile  = "invoice.draft.html"
	emailOperatorFile = "email.operator.draft.html"
	emailCspFile      = "email.csp.draft.html"
)

var (
	invoiceFuncMap = template.FuncMap{
		"seq": func(a, b int) []int {
			if b < a {
				return nil
			}
			s := make([]int, b-a+1)
			for i := range s {
				s[i] = a + i
			}
			return s
		},
		"dec": func(x int) int { return x - 1 },
		"indexOrEmpty": func(ss []string, i int) string {
			if i >= 0 && i < len(ss) {
				return ss[i]
			}
			return ""
		},
	}
)

func LoadConfirmEmailTemplate() (*template.Template, error) {
	return loadTemplate(emailConfirmFile)
}

func LoadKycFinalRejectedEmailTemplate() (*template.Template, error) {
	return loadTemplate(emailKycRejectedFile)
}

func LoadStepRejectedEmailTemplate() (*template.Template, error) {
	return loadTemplate(emailStepRejectedFile)
}

func LoadBlacklistedEmailTemplate() (*template.Template, error) {
	return loadTemplate(emailBlacklistedFile)
}

func LoadKycConfirmedEmailTemplate() (*template.Template, error) {
	return loadTemplate(emailKycConfirmedFile)
}

func LoadAccountResettedEmailTemplate() (*template.Template, error) {
	return loadTemplate(emailAccountResettedFile)
}

func LoadInvoiceDraftTemplate() (*template.Template, error) {
	return loadInvoiceTemplate(invoiceDraftFile)
}

func LoadOperatorDraftTemplate() (*template.Template, error) {
	return loadInvoiceTemplate(emailOperatorFile)
}

func LoadCspDraftTemplate() (*template.Template, error) {
	return loadInvoiceTemplate(emailCspFile)
}

func loadTemplate(filename string) (*template.Template, error) {
	templatePath, err := getTemplatePath(filename)
	if err != nil {
		return nil, err
	}
	parsedTemplate, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, err
	}

	return parsedTemplate, nil
}

func loadInvoiceTemplate(filename string) (*template.Template, error) {
	templatePath, err := getTemplatePath(filename)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(templatePath)
	return template.New(name).
		Option("missingkey=zero").
		Funcs(invoiceFuncMap).
		ParseFiles(templatePath)
}

func getTemplatePath(filename string) (string, error) {
	return config.Config.EmailTemplatesPath + filename, nil
}
