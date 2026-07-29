package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestCutoverApplyIntegration(t *testing.T) {
	if os.Getenv("RATIO1_CUTOVER_TEST_DATABASE") != "1" {
		t.Skip("set RATIO1_CUTOVER_TEST_DATABASE=1 to run the PostgreSQL integration test")
	}

	dsn := cutoverTestDatabaseDsn(t)
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer admin.Close()

	schema := "ratio1_cutover_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	})

	schemaDsn, err := url.Parse(dsn)
	require.NoError(t, err)
	query := schemaDsn.Query()
	query.Set("search_path", schema)
	schemaDsn.RawQuery = query.Encode()

	db, err := sql.Open("postgres", schemaDsn.String())
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE kycs (
		  uuid text PRIMARY KEY,
		  applicant_id text,
		  applicant_type text,
		  verification_provider text NOT NULL DEFAULT '',
		  email text NOT NULL UNIQUE,
		  kyc_status text NOT NULL,
		  last_updated timestamptz,
		  is_active boolean NOT NULL DEFAULT true,
		  has_been_deleted boolean NOT NULL DEFAULT false,
		  receive_updates boolean NOT NULL DEFAULT false,
		  country text,
		  vies_registered boolean NOT NULL DEFAULT false
		);
		CREATE TABLE user_infos (
		  blockchain_address text PRIMARY KEY,
		  email text
		);
		CREATE TABLE accounts (
		  address text PRIMARY KEY,
		  email text
		);
	`)
	require.NoError(t, err)

	emails := make([]string, 0, 4)
	for index, status := range []string{
		model.StatusApproved,
		model.StatusFinalRejected,
		model.StatusPending,
		model.StatusRejected,
	} {
		email := fmt.Sprintf("cutover-%d@example.test", index)
		emails = append(emails, email)
		_, err = db.Exec(
			`INSERT INTO kycs (
			   uuid, applicant_id, applicant_type, email, kyc_status, country, vies_registered
			 ) VALUES ($1, $2, $3, $4, $5, $6, TRUE)`,
			uuid.NewString(),
			fmt.Sprintf("sumsub-%d", index),
			model.IndividualCustomer,
			email,
			status,
			"ROU",
		)
		require.NoError(t, err)
		_, err = db.Exec(
			`INSERT INTO accounts (address, email) VALUES ($1, $2)`,
			fmt.Sprintf("0x%d", index+1),
			email,
		)
		require.NoError(t, err)
	}
	_, err = db.Exec(
		`INSERT INTO user_infos (blockchain_address, email)
		 VALUES ('0x1', $1), ('0x2', $2), ('0x3', $3)`,
		emails[0],
		emails[1],
		emails[2],
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, runCutover(ctx, db, cutoverOptions{
		Apply:                  true,
		ExpectedTotal:          4,
		ExpectedPreserved:      2,
		ExpectedReset:          2,
		ExpectedResetUserInfos: 1,
	}, io.Discard))

	var preserved, reset, userInfos, resetWithLegacyFields int64
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM kycs
		 WHERE kyc_status IN ($1, $2) AND verification_provider = $3`,
		model.StatusApproved,
		model.StatusFinalRejected,
		model.VerificationProviderSumsub,
	).Scan(&preserved))
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM kycs
		 WHERE kyc_status = $1
		   AND verification_provider = $2
		   AND applicant_id = ''
		   AND applicant_type = ''
		   AND country = ''
		   AND vies_registered = FALSE`,
		model.StatusAccountCreated,
		model.VerificationProviderDidit,
	).Scan(&reset))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_infos`).Scan(&userInfos))
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM kycs
		 WHERE kyc_status = $1
		   AND (applicant_id <> '' OR applicant_type <> '' OR country <> '' OR vies_registered)`,
		model.StatusAccountCreated,
	).Scan(&resetWithLegacyFields))

	require.Equal(t, int64(2), preserved)
	require.Equal(t, int64(2), reset)
	require.Equal(t, int64(2), userInfos)
	require.Zero(t, resetWithLegacyFields)
}

func cutoverTestDatabaseDsn(t *testing.T) string {
	t.Helper()
	host := envOr("RATIO1_TEST_DATABASE_HOST", "127.0.0.1")
	port := envOr("RATIO1_TEST_DATABASE_PORT", "5432")
	user := envOr("RATIO1_TEST_DATABASE_USER", "postgres")
	password := envOr("RATIO1_TEST_DATABASE_PASSWORD", "postgres")
	database := envOr("RATIO1_TEST_DATABASE_NAME", "ratio1_test")
	if !isLoopbackHost(host) {
		t.Fatal("cutover integration tests only permit a loopback PostgreSQL host")
	}
	return (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   database,
		RawQuery: url.Values{
			"sslmode": {"disable"},
		}.Encode(),
	}).String()
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
