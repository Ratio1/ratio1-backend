package service

import (
	"strings"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newTestWalletAddress(t *testing.T) string {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	return crypto.PubkeyToAddress(key.PublicKey).Hex()
}

func countAccounts(t *testing.T) int64 {
	t.Helper()
	db, err := storage.GetDB()
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&model.Account{}).Count(&count).Error)
	return count
}

func createKybTestAccount(t *testing.T, applicantType, kycStatus string) string {
	t.Helper()
	db, err := storage.GetDB()
	require.NoError(t, err)
	address := newTestWalletAddress(t)
	email := strings.ToLower(uuid.NewString()) + "@example.test"
	require.NoError(t, storage.CreateAccount(&model.Account{
		Address: address, Email: &email, EmailConfirmed: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}))
	require.NoError(t, db.Create(&model.Kyc{
		Uuid: uuid.New(), Email: email, ApplicantType: applicantType, KycStatus: kycStatus, LastUpdated: time.Now(),
	}).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
		require.NoError(t, db.Where("address = ?", address).Delete(&model.Account{}).Error)
	})
	return address
}

func TestIsWalletKyb_ShouldNotCreateAccountForUnknownWallet(t *testing.T) {
	requireServiceTestDatabase(t)
	before := countAccounts(t)
	address := newTestWalletAddress(t)

	isKyb, err := IsWalletKyb(strings.ToLower(address))

	require.Error(t, err)
	require.False(t, isKyb)
	require.Equal(t, before, countAccounts(t))
	_, found, err := storage.GetAccountByAddress(address)
	require.NoError(t, err)
	require.False(t, found)
}

func TestIsWalletKyb_ShouldRejectInvalidAddressWithoutWriting(t *testing.T) {
	requireServiceTestDatabase(t)
	before := countAccounts(t)
	for _, input := range []string{
		"", "notanaddress", "0x' OR 1=1--", "0xab", "0xab\x00cd",
		"0x15ca715eaf454d99b583b93cb6dccc70", strings.Repeat("a", 100),
	} {
		isKyb, err := IsWalletKyb(input)
		require.ErrorIs(t, err, ErrorInvalidWalletAddress, "input %q", input)
		require.False(t, isKyb)
	}
	require.Equal(t, before, countAccounts(t))
}

func TestIsWalletKyb_ShouldResolveAnyCasingToTheStoredAccount(t *testing.T) {
	requireServiceTestDatabase(t)
	company := createKybTestAccount(t, model.BusinessCustomer, model.StatusApproved)
	individual := createKybTestAccount(t, model.IndividualCustomer, model.StatusApproved)
	pending := createKybTestAccount(t, model.BusinessCustomer, "pending")
	before := countAccounts(t)

	for _, input := range []string{company, strings.ToLower(company), "0x" + strings.ToUpper(company[2:])} {
		isKyb, err := IsWalletKyb(input)
		require.NoError(t, err, "input %q", input)
		require.True(t, isKyb, "input %q", input)
	}

	isKyb, err := IsWalletKyb(individual)
	require.NoError(t, err)
	require.False(t, isKyb)

	_, err = IsWalletKyb(pending)
	require.Error(t, err)
	require.Equal(t, before, countAccounts(t))
}
