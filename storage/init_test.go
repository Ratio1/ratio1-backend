package storage

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var storageTestDatabaseAvailable bool

func TestMain(m *testing.M) {
	if os.Getenv("RATIO1_STORAGE_TEST_DATABASE") != "1" {
		os.Exit(m.Run())
	}

	host := envOrDefault("RATIO1_TEST_DATABASE_HOST", "127.0.0.1")
	if !isLoopbackTestHost(host) && os.Getenv("RATIO1_STORAGE_TEST_ALLOW_REMOTE") != "1" {
		fmt.Fprintln(os.Stderr, "storage tests refuse a non-local database host")
		os.Exit(1)
	}
	if !isLoopbackTestHost(host) && !isSecureRemoteTestSslMode(envOrDefault("RATIO1_TEST_DATABASE_SSLMODE", "require")) {
		fmt.Fprintln(os.Stderr, "remote storage tests require sslmode=require, verify-ca, or verify-full")
		os.Exit(1)
	}

	port := envOrDefault("RATIO1_TEST_DATABASE_PORT", "5432")
	if _, err := strconv.Atoi(port); err != nil {
		fmt.Fprintln(os.Stderr, "invalid storage test database port:", err)
		os.Exit(1)
	}

	user := envOrDefault("RATIO1_TEST_DATABASE_USER", "postgres")
	password := envOrDefault("RATIO1_TEST_DATABASE_PASSWORD", "postgres")
	dbName := envOrDefault("RATIO1_TEST_DATABASE_NAME", "ratio1_test")
	if !strings.Contains(strings.ToLower(dbName), "test") {
		fmt.Fprintln(os.Stderr, "storage test database name must contain 'test'")
		os.Exit(1)
	}

	adminDsn := postgresTestDsn(host, port, user, password, dbName, "")
	adminDb, err := sql.Open("postgres", adminDsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open storage test database:", err)
		os.Exit(1)
	}

	schemaName := "ratio1_storage_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = adminDb.Exec("CREATE SCHEMA " + schemaName); err != nil {
		_ = adminDb.Close()
		fmt.Fprintln(os.Stderr, "create storage test schema:", err)
		os.Exit(1)
	}

	testDsn := postgresTestDsn(host, port, user, password, dbName, schemaName)
	sqlDb, err := sql.Open("postgres", testDsn)
	if err != nil {
		_, _ = adminDb.Exec("DROP SCHEMA " + schemaName + " CASCADE")
		_ = adminDb.Close()
		fmt.Fprintln(os.Stderr, "open isolated storage test database:", err)
		os.Exit(1)
	}
	sqlDb.SetMaxOpenConns(10)
	sqlDb.SetMaxIdleConns(10)

	database, err = gorm.Open(postgres.New(postgres.Config{Conn: sqlDb}))
	if err == nil {
		err = TryMigrate()
	}
	if err != nil {
		_ = sqlDb.Close()
		_, _ = adminDb.Exec("DROP SCHEMA " + schemaName + " CASCADE")
		_ = adminDb.Close()
		fmt.Fprintln(os.Stderr, "migrate isolated storage test database:", err)
		os.Exit(1)
	}

	storageTestDatabaseAvailable = true
	code := m.Run()
	storageTestDatabaseAvailable = false

	_ = sqlDb.Close()
	if _, err = adminDb.Exec("DROP SCHEMA " + schemaName + " CASCADE"); err != nil {
		fmt.Fprintln(os.Stderr, "drop storage test schema:", err)
		code = 1
	}
	_ = adminDb.Close()
	os.Exit(code)
}

func requireStorageTestDatabase(t *testing.T) {
	t.Helper()
	if !storageTestDatabaseAvailable {
		t.Skip("set RATIO1_STORAGE_TEST_DATABASE=1 to run PostgreSQL storage tests")
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func postgresTestDsn(host, port, user, password, dbName, schemaName string) string {
	sslMode := "disable"
	if !isLoopbackTestHost(host) {
		sslMode = envOrDefault("RATIO1_TEST_DATABASE_SSLMODE", "require")
	}
	query := url.Values{"sslmode": []string{sslMode}}
	if schemaName != "" {
		query.Set("search_path", schemaName)
	}

	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     dbName,
		RawQuery: query.Encode(),
	}).String()
}

func isLoopbackTestHost(host string) bool {
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func isSecureRemoteTestSslMode(sslMode string) bool {
	switch sslMode {
	case "require", "verify-ca", "verify-full":
		return true
	default:
		return false
	}
}

func TestSecureRemoteTestSslMode(t *testing.T) {
	for _, sslMode := range []string{"require", "verify-ca", "verify-full"} {
		t.Run(sslMode, func(t *testing.T) {
			if !isSecureRemoteTestSslMode(sslMode) {
				t.Fatalf("expected %q to be accepted", sslMode)
			}
		})
	}

	for _, sslMode := range []string{"disable", "allow", "prefer", ""} {
		t.Run("reject_"+sslMode, func(t *testing.T) {
			if isSecureRemoteTestSslMode(sslMode) {
				t.Fatalf("expected %q to be rejected", sslMode)
			}
		})
	}
}
