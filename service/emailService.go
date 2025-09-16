package service

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/crypto"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/templates"
)

const (
	emailSendEndpoint      = "/email"
	emailSendBatchEndpoint = "/email/batch"

	messageStreamSend            = "outbound"
	subjectEmailConfirm          = "Ratio1 - Confirm your Email"
	subjectEmailKycFinalRejected = "Sorry - You did not pass the Ratio1 KYC"
	subjectEmailKycConfirmed     = "Congratulations - Ratio1 Technical KYC Completed"
	subjectEmailStepRejected     = "Check your KYC documentation uploads"
	subjectEmailAccountResetted  = "Your KYC has been resetted"
	subjectAddressBlacklisted    = "Address is blacklisted"
	subjectNewBuyLicenseInvoice  = "A new buy license invoice has been sent"
	subjectNewInvoiceDraft       = "A new invoice draft has been generated"
)

var (
	postmarkHeaders = func() []process.HttpHeaderPair {
		return []process.HttpHeaderPair{
			{
				Key:   "X-Postmark-Server-Token",
				Value: config.Config.Mail.ApiKey,
			},
		}
	}
)

type EmailSendResponse struct {
	ErrorCode   int       `json:"ErrorCode"`
	Message     string    `json:"Message"`
	MessageID   string    `json:"MessageID"`
	SubmittedAt time.Time `json:"SubmittedAt"`
	To          string    `json:"To"`
}

type EmailMessage struct {
	From          string `json:"From"`
	To            string `json:"To"`
	Subject       string `json:"Subject"`
	TextBody      string `json:"TextBody"`
	HtmlBody      string `json:"HtmlBody"`
	MessageStream string `json:"MessageStream"`
}

func SendNewsEmail(email []string, subject, htmlBody string) error {
	return callSendBatchEmail(email, subject, htmlBody)
}

func SendConfirmEmail(address, email string) error {
	template, err := templates.GetConfirmEmailTemplate()
	if err != nil {
		return errors.New("error while retrieving email template: " + err.Error())
	}

	token, err := crypto.GenerateConfirmJwt(
		address,
		email,
		config.Config.Jwt.ConfirmSecret,
		config.Config.Jwt.Issuer,
		config.Config.Jwt.ConfirmExpiryMins,
	)
	if err != nil {
		return errors.New("error while creating jwt: " + err.Error())
	}

	var body bytes.Buffer
	err = template.Execute(&body, struct{ Url string }{Url: confirmUrl(token)})
	if err != nil {
		return errors.New("error while executing email template: " + err.Error())
	}

	return callSendEmail(email, subjectEmailConfirm, body.String())
}

func SendKycFinalRejectedEmail(email string) error {
	template, err := templates.GetFinalRejectedEmailTemplate()
	if err != nil {
		return errors.New("error while retrieving email template: " + err.Error())
	}

	var body bytes.Buffer
	err = template.Execute(&body, struct{}{})
	if err != nil {
		return errors.New("error while executing email template: " + err.Error())
	}

	return callSendEmail(email, subjectEmailKycFinalRejected, body.String())
}

func SendKycConfirmedEmail(email string) error {
	template, err := templates.GetKycConfirmedEmailTemplate()
	if err != nil {
		return errors.New("error while retrieving email template: " + err.Error())
	}

	var body bytes.Buffer
	err = template.Execute(&body, struct{}{})
	if err != nil {
		return errors.New("error while executing email template: " + err.Error())
	}

	return callSendEmail(email, subjectEmailKycConfirmed, body.String())
}

func SendStepRejectedEmail(email string) error {
	template, err := templates.GetStepRejectedEmailTemplate()
	if err != nil {
		return errors.New("error while retrieving email template: " + err.Error())
	}

	var body bytes.Buffer
	err = template.Execute(&body, struct{}{})
	if err != nil {
		return errors.New("error while executing email template: " + err.Error())
	}

	return callSendEmail(email, subjectEmailStepRejected, body.String())
}

func SendBlacklistedEmail(email string) error {
	template, err := templates.GetBlacklistedEmailTemplate()
	if err != nil {
		return errors.New("error while retrieving email template: " + err.Error())
	}

	var body bytes.Buffer
	err = template.Execute(&body, struct{}{})
	if err != nil {
		return errors.New("error while executing email template: " + err.Error())
	}

	return callSendEmail(email, subjectAddressBlacklisted, body.String())
}

func SendAccountResettedEmail(email string) error {
	template, err := templates.GetAccountResettedEmailTemplate()
	if err != nil {
		return errors.New("error while retrieving email template: " + err.Error())
	}

	var body bytes.Buffer
	err = template.Execute(&body, struct{}{})
	if err != nil {
		return errors.New("error while executing email template: " + err.Error())
	}

	return callSendEmail(email, subjectEmailAccountResetted, body.String())
}

func SendNodeOwnerDraftEmail(email string) error {
	template, err := templates.GetOperatorDraftTemplate()
	if err != nil {
		return errors.New("error while retrieving email template: " + err.Error())
	}

	var body bytes.Buffer
	err = template.Execute(&body, struct{ Url string }{Url: config.Config.Ratio1redirectUrl.OperatorUrl})
	if err != nil {
		return errors.New("error while executing email template: " + err.Error())
	}
	return callSendEmail(email, subjectNewInvoiceDraft, body.String())
}

func SendCspDraftEmail(email string) error {
	template, err := templates.GetCspDraftTemplate()
	if err != nil {
		return errors.New("error while retrieving email template: " + err.Error())
	}

	var body bytes.Buffer
	err = template.Execute(&body, struct{ Url string }{Url: confirmUrl(config.Config.Ratio1redirectUrl.CspUrl)})
	if err != nil {
		return errors.New("error while executing email template: " + err.Error())
	}

	return callSendEmail(email, subjectNewInvoiceDraft, body.String())
}

func SendBuyLicenseEmail(email, url, invoiceNumber string) error {
	text := "A new invoice has been submitted. Invoice Number: " + invoiceNumber + " , link: " + url
	return callSendTextEmail(email, subjectNewBuyLicenseInvoice, text)
}

func callSendTextEmail(email, subject, text string) error {
	msg := EmailMessage{
		From:          config.Config.Mail.FromEmail,
		To:            email,
		Subject:       subject,
		TextBody:      text,
		HtmlBody:      "",
		MessageStream: messageStreamSend,
	}

	var resp EmailSendResponse
	url := fmt.Sprintf("%s%s", config.Config.Mail.ApiUrl, emailSendEndpoint)
	err := process.HttpPost(url, msg, &resp, postmarkHeaders()...)
	if err != nil {
		return err
	}
	if resp.ErrorCode != 0 {
		return fmt.Errorf("send email resulted in error %s", resp.Message)
	}

	return nil
}

func callSendBatchEmail(emails []string, subject, htmlBody string) error {
	var msg []EmailMessage
	for _, email := range emails {
		msg = append(msg, EmailMessage{
			From:          config.Config.Mail.FromEmail,
			To:            email,
			Subject:       subject,
			TextBody:      "",
			HtmlBody:      htmlBody,
			MessageStream: messageStreamSend,
		})
	}

	var resp []EmailSendResponse
	url := fmt.Sprintf("%s%s", config.Config.Mail.ApiUrl, emailSendBatchEndpoint)
	err := process.HttpPost(url, msg, &resp, postmarkHeaders()...)
	if err != nil {
		return err
	}
	for _, r := range resp {
		if r.ErrorCode != 0 {
			return fmt.Errorf("send email resulted in error %s", r.Message)
		}
	}

	return nil
}

func callSendEmail(email, subject, htmlBody string) error {
	msg := EmailMessage{
		From:          config.Config.Mail.FromEmail,
		To:            email,
		Subject:       subject,
		TextBody:      "",
		HtmlBody:      htmlBody,
		MessageStream: messageStreamSend,
	}

	var resp EmailSendResponse
	url := fmt.Sprintf("%s%s", config.Config.Mail.ApiUrl, emailSendEndpoint)
	err := process.HttpPost(url, msg, &resp, postmarkHeaders()...)
	if err != nil {
		return err
	}
	if resp.ErrorCode != 0 {
		return fmt.Errorf("send email resulted in error %s", resp.Message)
	}

	return nil
}

func confirmUrl(t string) string {
	return fmt.Sprintf(config.Config.Mail.ConfirmUrl, t)
}
