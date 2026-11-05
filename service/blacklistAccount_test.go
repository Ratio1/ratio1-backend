package service

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBlacklistAccountPersistsDespiteNotificationFailure(t *testing.T) {
	requireServiceTestDatabase(t)
	db, err := storage.GetDB()
	require.NoError(t, err)
	originalMail := config.Config.Mail
	t.Cleanup(func() { config.Config.Mail = originalMail })
	var notifications atomic.Int32
	var persistedBeforeNotification atomic.Bool
	address := uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notifications.Add(1)
		stored, found, err := storage.GetAccountByAddress(address)
		persistedBeforeNotification.Store(err == nil && found && stored.IsBlacklisted)
		http.Error(w, "test notification failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	config.Config.Mail.ApiUrl = server.URL
	config.Config.Mail.ApiKey = "test-token"
	email := address + "@example.test"
	account := &model.Account{Address: address, Email: &email, EmailConfirmed: true}
	require.NoError(t, storage.CreateAccount(account))
	t.Cleanup(func() { require.NoError(t, db.Where("address = ?", address).Delete(&model.Account{}).Error) })

	result, err := BlacklistAccount(address, "security review")
	require.NoError(t, err)
	require.True(t, result.IsBlacklisted)
	require.Equal(t, int32(1), notifications.Load())
	require.True(t, persistedBeforeNotification.Load())
	stored, found, err := storage.GetAccountByAddress(address)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, stored.IsBlacklisted)
	require.Equal(t, "security review", *stored.BlacklistedReason)
	require.Equal(t, &email, stored.Email)
	require.True(t, stored.EmailConfirmed)
}

func TestBlacklistAccountWithoutEmail(t *testing.T) {
	requireServiceTestDatabase(t)
	db, err := storage.GetDB()
	require.NoError(t, err)
	address := uuid.NewString()
	t.Cleanup(func() { require.NoError(t, db.Where("address = ?", address).Delete(&model.Account{}).Error) })
	result, err := BlacklistAccount(address, "blocked before registration")
	require.NoError(t, err)
	require.True(t, result.IsBlacklisted)
	require.Nil(t, result.Email)
	stored, found, err := storage.GetAccountByAddress(address)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, stored.IsBlacklisted)
	require.Equal(t, "blocked before registration", *stored.BlacklistedReason)
}
