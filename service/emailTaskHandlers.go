package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const (
	emailTaskSendConfirmEmail     = "send_confirm_email"
	emailTaskSendErrorEmail       = "send_error_email"
	emailTaskSendJobsEndingEmail  = "send_jobs_ending_email"
	emailTaskSendNodeOwnerDraft   = "send_node_owner_draft_email"
	emailTaskSendCspDraft         = "send_csp_draft_email"
	emailTaskSendBuyLicenseEmail  = "send_buy_license_email"
	emailTaskSendKycFinalRejected = "send_kyc_final_rejected_email"
	emailTaskSendKycStepRejected  = "send_kyc_step_rejected_email"
	emailTaskSendKycConfirmed     = "send_kyc_confirmed_email"
	emailTaskSendAccountResetted  = "send_account_resetted_email"
	emailTaskSendBlacklistedEmail = "send_blacklisted_email"
	emailTaskSendNewsletterBatch  = "send_newsletter_batch_email"
)

type emailTaskHandler func(task EmailTask) error

var (
	emailTaskHandlersMu sync.RWMutex
	emailTaskHandlers   = defaultEmailTaskHandlers()
)

func defaultEmailTaskHandlers() map[string]emailTaskHandler {
	return map[string]emailTaskHandler{
		emailTaskSendConfirmEmail:     handleSendConfirmEmailTask,
		emailTaskSendErrorEmail:       handleSendErrorEmailTask,
		emailTaskSendJobsEndingEmail:  handleSendJobsEndingEmailTask,
		emailTaskSendNodeOwnerDraft:   handleSendNodeOwnerDraftEmailTask,
		emailTaskSendCspDraft:         handleSendCspDraftEmailTask,
		emailTaskSendBuyLicenseEmail:  handleSendBuyLicenseEmailTask,
		emailTaskSendKycFinalRejected: handleSendKycFinalRejectedEmailTask,
		emailTaskSendKycStepRejected:  handleSendKycStepRejectedEmailTask,
		emailTaskSendKycConfirmed:     handleSendKycConfirmedEmailTask,
		emailTaskSendAccountResetted:  handleSendAccountResettedEmailTask,
		emailTaskSendBlacklistedEmail: handleSendBlacklistedEmailTask,
		emailTaskSendNewsletterBatch:  handleSendNewsletterBatchEmailTask,
	}
}

func getEmailTaskHandler(name string) (emailTaskHandler, bool) {
	emailTaskHandlersMu.RLock()
	defer emailTaskHandlersMu.RUnlock()

	handler, found := emailTaskHandlers[name]
	return handler, found
}

func registerEmailTaskHandler(name string, handler emailTaskHandler) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || handler == nil {
		return
	}

	emailTaskHandlersMu.Lock()
	defer emailTaskHandlersMu.Unlock()

	emailTaskHandlers[trimmed] = handler
}

func resetEmailTaskHandlersForTest() {
	emailTaskHandlersMu.Lock()
	defer emailTaskHandlersMu.Unlock()

	emailTaskHandlers = defaultEmailTaskHandlers()
}

type sendConfirmEmailPayload struct {
	Address string `json:"address"`
	Email   string `json:"email"`
}

type sendErrorEmailPayload struct {
	Message       string            `json:"message"`
	OriginalError string            `json:"originalError,omitempty"`
	Fields        []ErrorEmailField `json:"fields,omitempty"`
}

type sendJobsEndingEmailPayload struct {
	Recipient string      `json:"recipient"`
	Jobs      []EndingJob `json:"jobs"`
}

type sendSingleRecipientPayload struct {
	Recipient string `json:"recipient"`
}

type sendBuyLicenseEmailPayload struct {
	Recipient     string `json:"recipient"`
	URL           string `json:"url"`
	InvoiceNumber string `json:"invoiceNumber"`
}

type sendNewsletterBatchPayload struct {
	Recipients []string `json:"recipients"`
	Subject    string   `json:"subject"`
	HTMLBody   string   `json:"htmlBody"`
}

func NewSendConfirmEmailTask(address, email string) EmailTask {
	return EmailTask{
		Name: emailTaskSendConfirmEmail,
		Payload: sendConfirmEmailPayload{
			Address: address,
			Email:   email,
		},
	}
}

func NewSendErrorEmailTask(message string, originalErr error, fields []ErrorEmailField) EmailTask {
	payload := sendErrorEmailPayload{
		Message: message,
		Fields:  append([]ErrorEmailField(nil), fields...),
	}
	if originalErr != nil {
		payload.OriginalError = originalErr.Error()
	}

	return EmailTask{
		Name:    emailTaskSendErrorEmail,
		Payload: payload,
	}
}

func NewSendJobsEndingEmailTask(recipient string, jobs []EndingJob) EmailTask {
	return EmailTask{
		Name: emailTaskSendJobsEndingEmail,
		Payload: sendJobsEndingEmailPayload{
			Recipient: recipient,
			Jobs:      append([]EndingJob(nil), jobs...),
		},
	}
}

func NewSendNodeOwnerDraftEmailTask(recipient string) EmailTask {
	return EmailTask{
		Name: emailTaskSendNodeOwnerDraft,
		Payload: sendSingleRecipientPayload{
			Recipient: recipient,
		},
	}
}

func NewSendCspDraftEmailTask(recipient string) EmailTask {
	return EmailTask{
		Name: emailTaskSendCspDraft,
		Payload: sendSingleRecipientPayload{
			Recipient: recipient,
		},
	}
}

func NewSendBuyLicenseEmailTask(recipient, invoiceURL, invoiceNumber string) EmailTask {
	return EmailTask{
		Name: emailTaskSendBuyLicenseEmail,
		Payload: sendBuyLicenseEmailPayload{
			Recipient:     recipient,
			URL:           invoiceURL,
			InvoiceNumber: invoiceNumber,
		},
	}
}

func NewSendKycFinalRejectedEmailTask(email string) EmailTask {
	return EmailTask{
		Name: emailTaskSendKycFinalRejected,
		Payload: sendSingleRecipientPayload{
			Recipient: email,
		},
	}
}

func NewSendKycStepRejectedEmailTask(email string) EmailTask {
	return EmailTask{
		Name: emailTaskSendKycStepRejected,
		Payload: sendSingleRecipientPayload{
			Recipient: email,
		},
	}
}

func NewSendKycConfirmedEmailTask(email string) EmailTask {
	return EmailTask{
		Name: emailTaskSendKycConfirmed,
		Payload: sendSingleRecipientPayload{
			Recipient: email,
		},
	}
}

func NewSendAccountResettedEmailTask(email string) EmailTask {
	return EmailTask{
		Name: emailTaskSendAccountResetted,
		Payload: sendSingleRecipientPayload{
			Recipient: email,
		},
	}
}

func NewSendBlacklistedEmailTask(email string) EmailTask {
	return EmailTask{
		Name: emailTaskSendBlacklistedEmail,
		Payload: sendSingleRecipientPayload{
			Recipient: email,
		},
	}
}

func NewSendNewsletterBatchEmailTask(recipients []string, subject, htmlBody string) EmailTask {
	return EmailTask{
		Name: emailTaskSendNewsletterBatch,
		Payload: sendNewsletterBatchPayload{
			Recipients: append([]string(nil), recipients...),
			Subject:    subject,
			HTMLBody:   htmlBody,
		},
	}
}

func handleSendConfirmEmailTask(task EmailTask) error {
	var payload sendConfirmEmailPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendConfirmEmail(payload.Address, payload.Email)
}

func handleSendErrorEmailTask(task EmailTask) error {
	var payload sendErrorEmailPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendErrorEmail(payload.Message, emailTaskErrorFromString(payload.OriginalError), payload.Fields...)
}

func handleSendJobsEndingEmailTask(task EmailTask) error {
	var payload sendJobsEndingEmailPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendJobsEndingEmail(payload.Recipient, payload.Jobs)
}

func handleSendNodeOwnerDraftEmailTask(task EmailTask) error {
	var payload sendSingleRecipientPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendNodeOwnerDraftEmail(payload.Recipient)
}

func handleSendCspDraftEmailTask(task EmailTask) error {
	var payload sendSingleRecipientPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendCspDraftEmail(payload.Recipient)
}

func handleSendBuyLicenseEmailTask(task EmailTask) error {
	var payload sendBuyLicenseEmailPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendBuyLicenseEmail(payload.Recipient, payload.URL, payload.InvoiceNumber)
}

func handleSendKycFinalRejectedEmailTask(task EmailTask) error {
	var payload sendSingleRecipientPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendKycFinalRejectedEmail(payload.Recipient)
}

func handleSendKycStepRejectedEmailTask(task EmailTask) error {
	var payload sendSingleRecipientPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendStepRejectedEmail(payload.Recipient)
}

func handleSendKycConfirmedEmailTask(task EmailTask) error {
	var payload sendSingleRecipientPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendKycConfirmedEmail(payload.Recipient)
}

func handleSendAccountResettedEmailTask(task EmailTask) error {
	var payload sendSingleRecipientPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendAccountResettedEmail(payload.Recipient)
}

func handleSendBlacklistedEmailTask(task EmailTask) error {
	var payload sendSingleRecipientPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendBlacklistedEmail(payload.Recipient)
}

func handleSendNewsletterBatchEmailTask(task EmailTask) error {
	var payload sendNewsletterBatchPayload
	if err := decodeEmailTaskPayload(task, &payload); err != nil {
		return err
	}
	return SendNewsEmail(payload.Recipients, payload.Subject, payload.HTMLBody)
}

func decodeEmailTaskPayload(task EmailTask, out any) error {
	if out == nil {
		return fmt.Errorf("nil payload decode target for task %s", task.Name)
	}

	raw, err := json.Marshal(task.Payload)
	if err != nil {
		return fmt.Errorf("cannot encode payload for task %s: %w", task.Name, err)
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("cannot decode payload for task %s: %w", task.Name, err)
	}

	return nil
}

func emailTaskErrorFromString(v string) error {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return errors.New(trimmed)
}
