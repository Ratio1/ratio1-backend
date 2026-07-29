package storage

import (
	"fmt"
	"strings"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUpdateAccountAndCreateKycRollsBackAccountOnKycIdentityConflict(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	email := fmt.Sprintf("account-kyc-conflict-%s@example.com", uuid.NewString())
	address := testAccountAddress()
	account := &model.Account{
		Address:               address,
		PendingEmail:          email,
		PendingReceiveUpdates: true,
	}
	require.NoError(t, db.Create(account).Error)

	receiveUpdates := false
	preservedKyc := &model.Kyc{
		Uuid:           uuid.New(),
		Email:          email,
		KycStatus:      model.StatusApproved,
		ReceiveUpdates: &receiveUpdates,
	}
	require.NoError(t, CreateOrUpdateKyc(preservedKyc))
	t.Cleanup(func() {
		require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
		require.NoError(t, db.Where("address = ?", address).Delete(&model.Account{}).Error)
	})

	account.Email = &email
	account.EmailConfirmed = true
	account.PendingEmail = ""
	account.PendingReceiveUpdates = false
	newKyc := &model.Kyc{
		Uuid:           uuid.New(),
		Email:          email,
		KycStatus:      model.StatusAccountCreated,
		ReceiveUpdates: &receiveUpdates,
	}

	err = UpdateAccountAndCreateKyc(account, newKyc)
	require.ErrorIs(t, err, ErrKycIdentityConflict)

	storedAccount, found, err := GetAccountByAddress(address)
	require.NoError(t, err)
	require.True(t, found)
	require.Nil(t, storedAccount.Email)
	require.False(t, storedAccount.EmailConfirmed)
	require.Equal(t, email, storedAccount.PendingEmail)
	require.True(t, storedAccount.PendingReceiveUpdates)

	storedKyc, found, err := GetKycByEmail(email)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, preservedKyc.Uuid, storedKyc.Uuid)
	require.Equal(t, model.StatusApproved, storedKyc.KycStatus)
}

func TestUpdateAccountAndCreateKycCommitsBothRecords(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	email := fmt.Sprintf("account-kyc-commit-%s@example.com", uuid.NewString())
	address := testAccountAddress()
	account := &model.Account{
		Address:               address,
		PendingEmail:          email,
		PendingReceiveUpdates: true,
	}
	require.NoError(t, db.Create(account).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
		require.NoError(t, db.Where("address = ?", address).Delete(&model.Account{}).Error)
	})

	account.Email = &email
	account.EmailConfirmed = true
	account.PendingEmail = ""
	receiveUpdates := account.PendingReceiveUpdates
	account.PendingReceiveUpdates = false
	kyc := &model.Kyc{
		Uuid:           uuid.New(),
		Email:          email,
		KycStatus:      model.StatusAccountCreated,
		ReceiveUpdates: &receiveUpdates,
	}

	require.NoError(t, UpdateAccountAndCreateKyc(account, kyc))

	storedAccount, found, err := GetAccountByAddress(address)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, storedAccount.Email)
	require.Equal(t, email, *storedAccount.Email)
	require.True(t, storedAccount.EmailConfirmed)
	require.Empty(t, storedAccount.PendingEmail)
	require.False(t, storedAccount.PendingReceiveUpdates)

	storedKyc, found, err := GetKycByEmail(email)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, kyc.Uuid, storedKyc.Uuid)
	require.Equal(t, model.StatusAccountCreated, storedKyc.KycStatus)
	require.NotNil(t, storedKyc.ReceiveUpdates)
	require.True(t, *storedKyc.ReceiveUpdates)
}

func testAccountAddress() string {
	return "0x" + strings.ReplaceAll(uuid.NewString(), "-", "") + "00000000"
}
