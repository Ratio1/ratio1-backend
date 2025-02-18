package service

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/crypto"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
)

var ErrorAccountNotFound = errors.New("account not found")

func GetOrCreateAccount(address string) (*model.Account, error) {
	account, err := getAcocunt(address)
	if err != nil {
		return nil, errors.New("error while retrieving account from storage: " + err.Error())
	}
	if account == nil {
		newAccount := &model.Account{Address: address, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		err = storage.CreateAccount(newAccount)
		if err != nil {
			return nil, errors.New("error while creating account in storage:" + err.Error())
		}
		return newAccount, nil
	}

	return account, nil
}

func RegisterEmail(address, email string, receiveUpdates bool) (*model.Account, error) {
	email = TrimWhitespacesAndToLower(email)
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, errors.New("invalid email address: " + email)
	}

	_, found, err := storage.GetAccountByEmail(email)
	if err != nil {
		return nil, errors.New("error while retrieving email from storage: " + err.Error())
	}
	if found {
		return nil, errors.New("email is already used")
	}

	account, err := getAcocunt(address)
	if err != nil {
		return nil, errors.New("error while retrieving account from storage: " + err.Error())
	} else if account == nil {
		return nil, ErrorAccountNotFound
	}

	if account.EmailConfirmed {
		return nil, errors.New("account already has another email")
	}

	account.PendingEmail = email
	account.PendingReceiveUpdates = receiveUpdates

	err = storage.UpdateAccount(account)
	if err != nil {
		return nil, errors.New("error while updating account on storage: " + err.Error())
	}

	err = increaseEmailCount(address)
	if err != nil {
		return nil, err
	}

	err = SendConfirmEmail(address, email)
	if err != nil {
		return nil, errors.New("error while sending confirmation email: " + err.Error())
	}

	return account, nil
}

func ConfirmEmail(token string) (*model.Account, error) {
	claims, err := crypto.ValidateConfirmJwt(token, config.Config.Jwt.ConfirmSecret)
	if err != nil {
		return nil, errors.New("error while validating confirm jwt: " + err.Error())
	}
	if claims.Address == "" && claims.Email == "" {
		return nil, errors.New("found bad claims in confirm token")
	}

	account, err := getAcocunt(claims.Address)
	if err != nil {
		return nil, errors.New("error while retrieving account from storage: " + err.Error())
	} else if account == nil {
		return nil, ErrorAccountNotFound
	}

	if account.EmailConfirmed {
		if *account.Email == claims.Email {
			return account, nil // already confirmed
		} else {
			return nil, errors.New("account already has another email")
		}
	}

	if account.PendingEmail != claims.Email {
		return nil, errors.New("wrong confirmation token")
	}

	account.Email = &claims.Email
	account.EmailConfirmed = true
	account.PendingEmail = ""
	receiveUpdates := account.PendingReceiveUpdates

	kyc := model.Kyc{
		Email:          *account.Email,
		Uuid:           uuid.New(),
		KycStatus:      model.StatusInit,
		ReceiveUpdates: &receiveUpdates,
	}
	account.PendingReceiveUpdates = false

	err = storage.UpdateAccount(account)
	if err != nil {
		return nil, errors.New("error while updating account on storage: " + err.Error())
	}

	err = storage.CreateOrUpdateKyc(&kyc)
	if err != nil {
		return nil, errors.New("error while updating kyc on storage: " + err.Error())
	}

	if *kyc.ReceiveUpdates {
		_ = AddSubscriber(*account.Email) // ignore error
	} else {
		_ = RemoveSubscriber(*account.Email) // ignore error
	}

	return account, nil
}

func SubscribeEmail(kyc *model.Kyc) error {
	if *kyc.ReceiveUpdates {
		return nil // already subscribed
	}

	var rUpd = true
	kyc.ReceiveUpdates = &rUpd

	err := storage.CreateOrUpdateKyc(kyc)
	if err != nil {
		return errors.New("error while update kyc in storage: " + err.Error())
	}

	_ = AddSubscriber(kyc.Email) // ignore error

	return nil
}

func UnsubscribeEmail(kyc *model.Kyc) error {
	if !*kyc.ReceiveUpdates {
		return nil // already unsubscribed
	}

	var rUpd = false
	kyc.ReceiveUpdates = &rUpd

	err := storage.CreateOrUpdateKyc(kyc)
	if err != nil {
		return errors.New("error while update kyc in storage: " + err.Error())
	}

	_ = RemoveSubscriber(kyc.Email) // ignore error

	return nil
}

func NewAccountDto(account *model.Account, kyc *model.Kyc) *model.AccountDto {
	if kyc == nil {
		return &model.AccountDto{
			Email:             StringOrEmpty(account.Email),
			EmailConfirmed:    account.EmailConfirmed,
			PendingEmail:      account.PendingEmail,
			Address:           account.Address,
			ApplicantType:     "",
			Uuid:              "",
			KycStatus:         "",
			ReceiveUpdates:    false,
			IsActive:          false,
			IsBlacklisted:     account.IsBlacklisted,
			BlacklistedReason: account.BlacklistedReason,
			UsdBuyLimit:       0,
		}
	}
	limit := 0
	if kyc.ApplicantType == model.BusinessCustomer {
		limit = config.Config.BuyLimitUSD.Company
	} else if kyc.ApplicantType == model.IndividualCustomer {
		limit = config.Config.BuyLimitUSD.Individual
	}
	return &model.AccountDto{
		Email:             StringOrEmpty(account.Email),
		EmailConfirmed:    account.EmailConfirmed,
		PendingEmail:      account.PendingEmail,
		Address:           account.Address,
		ApplicantType:     kyc.ApplicantType,
		Uuid:              kyc.Uuid.String(),
		KycStatus:         kyc.KycStatus,
		ReceiveUpdates:    *kyc.ReceiveUpdates,
		IsActive:          kyc.IsActive,
		IsBlacklisted:     account.IsBlacklisted,
		BlacklistedReason: account.BlacklistedReason,
		UsdBuyLimit:       limit,
	}
}

func getAcocunt(address string) (*model.Account, error) {
	storedAccount, found, err := storage.GetAccountByAddress(address)
	if err != nil {
		return nil, errors.New("error while retrieving account from storage: " + err.Error())
	}

	if !found {
		return nil, nil
	}
	return storedAccount, nil
}

func StringOrEmpty(str *string) string {
	if str != nil {
		return *str
	}

	return ""
}

func TrimWhitespacesAndToLower(emailAddress string) string {
	return strings.ToLower(strings.ReplaceAll(emailAddress, " ", ""))
}
