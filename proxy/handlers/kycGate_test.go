package handlers

import (
	"context"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/crypto"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	handlerTestDatabaseOnce sync.Once
	handlerTestDatabaseErr  error
	handlerTestKeyOnce      sync.Once
)

func handlerTestEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// requireHandlerTestDatabase uses the same opt-in and loopback-only rules as the
// service integration tests.
func requireHandlerTestDatabase(t *testing.T) {
	t.Helper()
	if os.Getenv("RATIO1_SERVICE_TEST_DATABASE") != "1" {
		t.Skip("set RATIO1_SERVICE_TEST_DATABASE=1 to run database-backed handler tests")
	}
	host := handlerTestEnvOrDefault("RATIO1_TEST_DATABASE_HOST", "127.0.0.1")
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		t.Fatal("handler integration tests require a loopback database host")
	}
	dbName := handlerTestEnvOrDefault("RATIO1_TEST_DATABASE_NAME", "ratio1_test")
	if !strings.Contains(strings.ToLower(dbName), "test") {
		t.Fatal("handler integration test database name must contain test")
	}
	port, err := strconv.Atoi(handlerTestEnvOrDefault("RATIO1_TEST_DATABASE_PORT", "5432"))
	require.NoError(t, err)
	handlerTestDatabaseOnce.Do(func() {
		databaseConfig := config.DatabaseConfig{
			Host:         host,
			Port:         port,
			User:         handlerTestEnvOrDefault("RATIO1_TEST_DATABASE_USER", "postgres"),
			Password:     handlerTestEnvOrDefault("RATIO1_TEST_DATABASE_PASSWORD", "postgres"),
			DbName:       dbName,
			SslMode:      "disable",
			MaxOpenConns: 10,
			MaxIdleConns: 10,
		}
		handlerTestDatabaseErr = storage.Migrate(context.Background(), databaseConfig)
		if handlerTestDatabaseErr != nil {
			return
		}
		config.Config.Database = databaseConfig
		storage.Connect()
	})
	require.NoError(t, handlerTestDatabaseErr)
}

// useTestBackendKey points NAEURAL_PEM_FILE at a throwaway secp256k1 key in the
// PKCS#8 layout service.LoadPrivateKeyFromPemFile reads. service caches the key
// for the process, so it is written once.
func useTestBackendKey(t *testing.T) {
	t.Helper()
	handlerTestKeyOnce.Do(func() {
		key, err := ethcrypto.GenerateKey()
		require.NoError(t, err)
		ecKey, err := asn1.Marshal(struct {
			Version    int
			PrivateKey []byte
		}{1, ethcrypto.FromECDSA(key)})
		require.NoError(t, err)
		pkcs8, err := asn1.Marshal(struct {
			Version             int
			PrivateKeyAlgorithm pkix.AlgorithmIdentifier
			PrivateKey          []byte
		}{0, pkix.AlgorithmIdentifier{Algorithm: asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}}, ecKey})
		require.NoError(t, err)
		path := filepath.Join(os.TempDir(), "ratio1-backend-handler-test-"+uuid.NewString()+".pem")
		require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), 0o600))
		require.NoError(t, os.Setenv("NAEURAL_PEM_FILE", path))
	})
	_, err := service.GetAddress()
	require.NoError(t, err)
}

func TestKycGate_ShouldRequireApprovedKycOnlyOnGatedRoutes(t *testing.T) {
	requireHandlerTestDatabase(t)
	useTestBackendKey(t)
	gin.SetMode(gin.TestMode)

	originalJwt, originalDevTesting := config.Config.Jwt, config.Config.Api.DevTesting
	t.Cleanup(func() { config.Config.Jwt, config.Config.Api.DevTesting = originalJwt, originalDevTesting })
	config.Config.Jwt.Secret = "handler-test-secret"
	config.Config.Api.DevTesting = false

	groups := NewGroupHandler()
	NewAccountHandler(groups)
	NewBrandingHandler(groups)
	NewInvoiceDraftHandler(groups)
	router := gin.New()
	groups.RegisterEndpoints(router)

	db, err := storage.GetDB()
	require.NoError(t, err)
	newWallet := func() string {
		key, err := ethcrypto.GenerateKey()
		require.NoError(t, err)
		return ethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	}
	unverified := newWallet()
	approved := newWallet()
	email := strings.ToLower(uuid.NewString()) + "@example.test"
	require.NoError(t, storage.CreateAccount(&model.Account{Address: approved, Email: &email, EmailConfirmed: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}))
	require.NoError(t, db.Create(&model.Kyc{Uuid: uuid.New(), Email: email, ApplicantType: model.IndividualCustomer, KycStatus: model.StatusApproved, IsActive: true, LastUpdated: time.Now()}).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
		require.NoError(t, db.Where("address IN ?", []string{approved, unverified}).Delete(&model.Account{}).Error)
	})

	call := func(method, path, address string) *httptest.ResponseRecorder {
		token, err := crypto.GenerateJwt(address, config.Config.Jwt.Secret, "handler-test", 5)
		require.NoError(t, err)
		request := httptest.NewRequest(method, path, strings.NewReader("{}"))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	gated := [][2]string{
		{http.MethodPost, "/branding/edit"},
		{http.MethodPost, "/branding/edit-logo"},
		{http.MethodPost, "/invoice-draft/create-preferences"},
		{http.MethodPost, "/invoice-draft/change-preferences"},
		{http.MethodPost, "/accounts/add-seller-code?sellerCode=ZZTEST"},
	}
	for _, route := range gated {
		rejected := call(route[0], route[1], unverified)
		require.Equal(t, http.StatusForbidden, rejected.Code, "%s %s", route[0], route[1])
		require.Contains(t, rejected.Body.String(), service.ErrorKycNotApproved.Error())

		admitted := call(route[0], route[1], approved)
		require.NotEqual(t, http.StatusForbidden, admitted.Code, "%s %s: %s", route[0], route[1], admitted.Body.String())
	}

	_, found, err := storage.GetAccountByAddress(unverified)
	require.NoError(t, err)
	require.False(t, found, "a refused gated request must not create an account")

	// Ungated authenticated reads still work for wallets without KYC.
	for _, route := range [][2]string{
		{http.MethodGet, "/invoice-draft/get-preferences"},
		{http.MethodGet, "/accounts/kyc-info"},
	} {
		require.NotEqual(t, http.StatusForbidden, call(route[0], route[1], unverified).Code, "%s %s", route[0], route[1])
	}

}
