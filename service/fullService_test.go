package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/crypto"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/stretchr/testify/require"
)

func Test_FullTest(t *testing.T) { //TO TEST THIS RETURN NIL ON emailService.go callSendEmail()
	address := "0xpipppoPippi"
	email := "alberto.bast29@gmail.com"

	account, err := GetOrCreateAccount(address)
	require.Nil(t, err)
	require.Equal(t, account.Address, address)
	_ = NewAccountDto(account, nil)

	expectedAccount := model.Account{
		Address:               address,
		PendingEmail:          email,
		Email:                 nil,
		PendingReceiveUpdates: false,
	}
	account, err = RegisterEmail(address, email, false)
	require.Nil(t, err)
	require.Equal(t, account.Address, expectedAccount.Address)
	require.Equal(t, account.PendingEmail, expectedAccount.PendingEmail)
	require.Equal(t, account.PendingReceiveUpdates, expectedAccount.PendingReceiveUpdates)
	require.Equal(t, account.Email, expectedAccount.Email)
	require.Equal(t, account.EmailConfirmed, false)
	_ = NewAccountDto(account, nil)

	token, err := crypto.GenerateConfirmJwt(
		address,
		email,
		config.Config.Jwt.ConfirmSecret,
		config.Config.Jwt.Issuer,
		config.Config.Jwt.ConfirmExpiryMins,
	)
	require.Nil(t, err)

	expectedAccount = model.Account{
		Address:               address,
		PendingEmail:          "",
		Email:                 &email,
		PendingReceiveUpdates: false,
	}

	account, err = ConfirmEmail(token)
	require.Nil(t, err)
	require.Equal(t, account.Address, expectedAccount.Address)
	require.Equal(t, account.PendingEmail, expectedAccount.PendingEmail)
	require.Equal(t, account.PendingReceiveUpdates, expectedAccount.PendingReceiveUpdates)
	require.Equal(t, account.Email, expectedAccount.Email)
	require.Equal(t, account.EmailConfirmed, true)
	_ = NewAccountDto(account, nil)

	_, err = RegisterEmail(address, email, false)
	require.Equal(t, err, errors.New("email is already used"))

	kyc, found, err := storage.GetKycByEmail(*account.Email)
	require.Nil(t, err)
	require.True(t, found)
	_ = NewAccountDto(account, kyc)
	err = SubscribeEmail(kyc)
	require.Nil(t, err)
	newKyc, found, err := storage.GetKycByEmail(*account.Email)
	require.Nil(t, err)
	require.True(t, found)
	require.True(t, *newKyc.ReceiveUpdates)
	_ = NewAccountDto(account, kyc)
	err = UnsubscribeEmail(newKyc)
	require.Nil(t, err)
	kyc, found, err = storage.GetKycByEmail(*account.Email)
	require.Nil(t, err)
	require.True(t, found)
	require.False(t, *kyc.ReceiveUpdates)
	_ = NewAccountDto(account, kyc)

	kycUuid := "166cdcfc-afec-4372-90a0-f718a8133414"
	//url, err := InitNewSession(kyc.Uuid.String(), model.IndividualCustomer)
	url, err := InitNewSession(kycUuid, model.IndividualCustomer)
	require.Nil(t, err)
	fmt.Println(*url)

	kyc, found, err = storage.GetKycByUuid(kyc.Uuid)
	require.Nil(t, err)
	require.True(t, found)
	event := model.SumsubEvent{
		ApplicantID:  "6799f6261e7b704966cde79b",
		InspectionID: "1",
		ReviewStatus: model.StatusPending,
		CreatedAtMs:  time.Now().Format("2006-01-02 15:04:05.000"),
	}
	err = ProcessKycEvent(event, *kyc)
	require.Nil(t, err)
	kyc, found, err = storage.GetKycByUuid(kyc.Uuid)
	require.Nil(t, err)
	require.True(t, found)
	require.Equal(t, kyc.KycStatus, model.StatusPending)
}
