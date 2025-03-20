package templates

import (
	"html/template"
	"sync"
)

var (
	once sync.Once

	confirmEmailTemplate          *template.Template
	blacklistedEmailTemplate      *template.Template
	kycConfirmedEmailTemplate     *template.Template
	rejectedStepEmailTemplate     *template.Template
	kycFinalRejectedEmailTemplate *template.Template
	accountResettedEmailTemplate  *template.Template
)

func LoadAndCacheTemplates() {
	once.Do(func() {
		confirm, err := LoadConfirmEmailTemplate()
		if err != nil {
			panic(err)
		}
		finalReject, err := LoadKycFinalRejectedEmailTemplate()
		if err != nil {
			panic(err)
		}
		stepReject, err := LoadStepRejectedEmailTemplate()
		if err != nil {
			panic(err)
		}
		blacklisted, err := LoadBlacklistedEmailTemplate()
		if err != nil {
			panic(err)
		}
		finalStatus, err := LoadKycConfirmedEmailTemplate()
		if err != nil {
			panic(err)
		}
		accountResetted, err := LoadAccountResettedEmailTemplate()
		if err != nil {
			panic(err)
		}

		confirmEmailTemplate = confirm
		blacklistedEmailTemplate = blacklisted
		kycConfirmedEmailTemplate = finalStatus
		rejectedStepEmailTemplate = stepReject
		kycFinalRejectedEmailTemplate = finalReject
		accountResettedEmailTemplate = accountResetted
	})
}

func GetConfirmEmailTemplate() (*template.Template, error) {
	return getOrSetTemplate(LoadConfirmEmailTemplate, confirmEmailTemplate)
}

func GetFinalRejectedEmailTemplate() (*template.Template, error) {
	return getOrSetTemplate(LoadKycFinalRejectedEmailTemplate, kycFinalRejectedEmailTemplate)
}

func GetStepRejectedEmailTemplate() (*template.Template, error) {
	return getOrSetTemplate(LoadStepRejectedEmailTemplate, rejectedStepEmailTemplate)
}

func GetBlacklistedEmailTemplate() (*template.Template, error) {
	return getOrSetTemplate(LoadBlacklistedEmailTemplate, blacklistedEmailTemplate)
}

func GetKycConfirmedEmailTemplate() (*template.Template, error) {
	return getOrSetTemplate(LoadKycConfirmedEmailTemplate, kycConfirmedEmailTemplate)
}

func GetAccountResettedEmailTemplate() (*template.Template, error) {
	return getOrSetTemplate(LoadAccountResettedEmailTemplate, accountResettedEmailTemplate)
}

func getOrSetTemplate(
	getter func() (*template.Template, error),
	target *template.Template,
) (*template.Template, error) {
	if target == nil {
		t, err := getter()
		if err != nil {
			return nil, err
		}
		target = t
		return t, nil
	}

	return target, nil
}
