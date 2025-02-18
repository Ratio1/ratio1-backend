package templates

import (
	"html/template"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
)

const (
	emailConfirmFile      = "email.confirm.html"
	emailKycRejectedFile  = "email.final.rejected.html"
	emailStepRejectedFile = "email.step.rejected.html"
	emailBlacklistedFile  = "email.blacklisted.html"
	emailKycConfirmedFile = "email.kyc.confirmed.html"
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

func getTemplatePath(filename string) (string, error) {
	return config.Config.EmailTemplatesPath + filename, nil
}
