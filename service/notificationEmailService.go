package service

import (
	"errors"
	"net/mail"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
)

func RegisterNotificationEmail(address, email string) (*model.Account, error) {
	email = TrimWhitespacesAndToLower(email)
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, errors.New("invalid email address: " + email)
	}

	account, err := getAcocunt(address)
	if err != nil {
		return nil, errors.New("error while retrieving account from storage: " + err.Error())
	} else if account == nil {
		return nil, ErrorAccountNotFound
	}
	if account.Email == nil || !account.EmailConfirmed {
		return nil, errors.New("default notification email is not confirmed")
	}
	if email == TrimWhitespacesAndToLower(*account.Email) || email == TrimWhitespacesAndToLower(account.PendingEmail) {
		return nil, errors.New("additional notification email must be different from default notification email")
	}

	notificationEmail, found, err := storage.GetAccountNotificationEmailByAddress(address)
	if err != nil {
		return nil, errors.New("error while retrieving notification email from storage: " + err.Error())
	}
	if !found {
		notificationEmail = &model.AccountNotificationEmail{
			AccountAddress: address,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
	}
	if notificationEmail.EmailConfirmed && notificationEmail.Email != nil && TrimWhitespacesAndToLower(*notificationEmail.Email) == email {
		return account, nil
	}

	notificationEmail.PendingEmail = email
	notificationEmail.UpdatedAt = time.Now()

	err = storage.CreateOrUpdateAccountNotificationEmail(notificationEmail)
	if err != nil {
		return nil, errors.New("error while updating notification email on storage: " + err.Error())
	}

	err = SendConfirmEmail(address, email)
	if err != nil {
		return nil, errors.New("error while sending confirmation email: " + err.Error())
	}

	return account, nil
}

func DeleteNotificationEmail(address string) (*model.Account, error) {
	account, err := getAcocunt(address)
	if err != nil {
		return nil, errors.New("error while retrieving account from storage: " + err.Error())
	} else if account == nil {
		return nil, ErrorAccountNotFound
	}

	err = storage.DeleteAccountNotificationEmail(address)
	if err != nil {
		return nil, errors.New("error while deleting notification email from storage: " + err.Error())
	}

	return account, nil
}

func confirmPendingNotificationEmail(notificationEmail *model.AccountNotificationEmail, email string) (bool, error) {
	if notificationEmail == nil {
		return false, nil
	}

	email = TrimWhitespacesAndToLower(email)
	if notificationEmail.EmailConfirmed && notificationEmail.Email != nil && TrimWhitespacesAndToLower(*notificationEmail.Email) == email {
		return true, nil
	}
	if TrimWhitespacesAndToLower(notificationEmail.PendingEmail) != email {
		return false, nil
	}

	notificationEmail.Email = &email
	notificationEmail.EmailConfirmed = true
	notificationEmail.PendingEmail = ""
	notificationEmail.UpdatedAt = time.Now()

	return true, nil
}

func notificationEmailsForAccount(account *model.Account, notificationEmail *model.AccountNotificationEmail) []string {
	emails := make([]string, 0, 2)
	seen := make(map[string]bool)

	addEmail := func(email string) {
		email = TrimWhitespacesAndToLower(email)
		if email == "" || seen[email] {
			return
		}
		seen[email] = true
		emails = append(emails, email)
	}

	if account != nil && account.EmailConfirmed && account.Email != nil {
		addEmail(*account.Email)
	}
	if notificationEmail != nil && notificationEmail.EmailConfirmed && notificationEmail.Email != nil {
		addEmail(*notificationEmail.Email)
	}

	return emails
}
