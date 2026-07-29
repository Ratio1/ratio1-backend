package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	_ "github.com/lib/pq"
)

const (
	databaseLinkEnvironment = "DATABASE_LINK"
	cutoverApplicationName  = "ratio1_verification_cutover"
)

type cutoverOptions struct {
	Apply                  bool
	ExpectedTotal          int64
	ExpectedPreserved      int64
	ExpectedReset          int64
	ExpectedResetUserInfos int64
}

type databaseConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
}

type cutoverAggregate struct {
	Total                    int64
	Approved                 int64
	FinalRejected            int64
	Preserved                int64
	Reset                    int64
	UserInfos                int64
	ResetUserInfos           int64
	ApprovedMissingUserInfos int64
	DuplicateUuidGroups      int64
	DuplicateEmailGroups     int64
	InvalidIdentityRows      int64
	ResetRowsWithLegacyData  int64
	Statuses                 map[string]int64
	Providers                map[string]int64
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validateOptions(opts cutoverOptions) error {
	expected := []int64{
		opts.ExpectedTotal,
		opts.ExpectedPreserved,
		opts.ExpectedReset,
		opts.ExpectedResetUserInfos,
	}
	provided := 0
	for _, value := range expected {
		if value >= 0 {
			provided++
		}
	}
	if opts.Apply && provided != len(expected) {
		return errors.New("--apply requires --expected-total, --expected-preserved, --expected-reset, and --expected-reset-user-infos")
	}
	if !opts.Apply && provided != 0 && provided != len(expected) {
		return errors.New("expected counts must either all be supplied or all be omitted")
	}
	if provided == len(expected) && opts.ExpectedPreserved+opts.ExpectedReset != opts.ExpectedTotal {
		return errors.New("expected preserved plus expected reset must equal expected total")
	}
	return nil
}

func loadDatabaseConfigFromEnvironment() (databaseConfig, error) {
	if databaseLink := os.Getenv(databaseLinkEnvironment); databaseLink != "" {
		return parseDatabaseLink(databaseLink, os.Getenv("DATABASE_SSLMODE"))
	}

	port, err := strconv.Atoi(os.Getenv("DATABASE_PORT"))
	if err != nil || port <= 0 {
		return databaseConfig{}, errors.New("DATABASE_PORT must be a positive integer")
	}
	cfg := databaseConfig{
		Host:     strings.TrimSpace(os.Getenv("DATABASE_HOST")),
		Port:     port,
		Database: strings.TrimSpace(os.Getenv("DATABASE_NAME")),
		User:     strings.TrimSpace(os.Getenv("DATABASE_USER")),
		Password: os.Getenv("DATABASE_PASSWORD"),
		SSLMode:  strings.TrimSpace(os.Getenv("DATABASE_SSLMODE")),
	}
	return validateDatabaseConfig(cfg)
}

func parseDatabaseLink(databaseLink, sslMode string) (databaseConfig, error) {
	parts := strings.SplitN(databaseLink, ":", 5)
	if len(parts) != 5 {
		return databaseConfig{}, errors.New("DATABASE_LINK must have host:port:database:user:password form")
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil || port <= 0 {
		return databaseConfig{}, errors.New("DATABASE_LINK port must be a positive integer")
	}
	return validateDatabaseConfig(databaseConfig{
		Host:     strings.TrimSpace(parts[0]),
		Port:     port,
		Database: strings.TrimSpace(parts[2]),
		User:     strings.TrimSpace(parts[3]),
		Password: parts[4],
		SSLMode:  strings.TrimSpace(sslMode),
	})
}

func validateDatabaseConfig(cfg databaseConfig) (databaseConfig, error) {
	if cfg.Host == "" || cfg.Database == "" || cfg.User == "" || cfg.Password == "" {
		return databaseConfig{}, errors.New("database host, name, user, and password are required")
	}
	if cfg.SSLMode == "" {
		if isLoopbackHost(cfg.Host) {
			cfg.SSLMode = "disable"
		} else {
			cfg.SSLMode = "require"
		}
	}
	if !isLoopbackHost(cfg.Host) && !isSecureSSLMode(cfg.SSLMode) {
		return databaseConfig{}, errors.New("non-loopback database connections require sslmode=require, verify-ca, or verify-full")
	}
	return cfg, nil
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isSecureSSLMode(sslMode string) bool {
	switch sslMode {
	case "require", "verify-ca", "verify-full":
		return true
	default:
		return false
	}
}

func openDatabase(cfg databaseConfig) (*sql.DB, error) {
	dsn := (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   cfg.Database,
		RawQuery: url.Values{
			"application_name": {cutoverApplicationName},
			"connect_timeout":  {"10"},
			"sslmode":          {cfg.SSLMode},
		}.Encode(),
	}).String()
	return sql.Open("postgres", dsn)
}

func runCutover(ctx context.Context, db *sql.DB, opts cutoverOptions, output io.Writer) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if !opts.Apply {
		return runDryRun(ctx, db, opts, output)
	}
	return runApply(ctx, db, opts, output)
}

func runDryRun(ctx context.Context, db *sql.DB, opts cutoverOptions, output io.Writer) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin read-only transaction: %w", err)
	}
	defer tx.Rollback()

	aggregate, err := collectCutoverAggregate(ctx, tx)
	if err != nil {
		return err
	}
	if err = validatePreCutoverAggregate(aggregate, opts); err != nil {
		printAggregate(output, "dry-run", aggregate)
		return err
	}
	printAggregate(output, "dry-run", aggregate)
	fmt.Fprintln(output, "result=ready_for_reviewed_apply")
	if err = tx.Rollback(); err != nil {
		return fmt.Errorf("end read-only transaction: %w", err)
	}
	return nil
}

func runApply(ctx context.Context, db *sql.DB, opts cutoverOptions, output io.Writer) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin serializable transaction: %w", err)
	}
	defer tx.Rollback()

	lockedRows, err := tx.QueryContext(ctx, `SELECT uuid FROM kycs ORDER BY uuid FOR UPDATE`)
	if err != nil {
		return fmt.Errorf("lock KYC rows: %w", err)
	}
	var lockedCount int64
	for lockedRows.Next() {
		var uuid string
		if err = lockedRows.Scan(&uuid); err != nil {
			lockedRows.Close()
			return fmt.Errorf("scan locked KYC row: %w", err)
		}
		lockedCount++
	}
	if err = lockedRows.Err(); err != nil {
		lockedRows.Close()
		return fmt.Errorf("iterate locked KYC rows: %w", err)
	}
	if err = lockedRows.Close(); err != nil {
		return fmt.Errorf("close locked KYC rows: %w", err)
	}

	before, err := collectCutoverAggregate(ctx, tx)
	if err != nil {
		return err
	}
	if lockedCount != before.Total {
		return fmt.Errorf("locked KYC count %d does not match aggregate total %d", lockedCount, before.Total)
	}
	if err = validatePreCutoverAggregate(before, opts); err != nil {
		printAggregate(output, "before", before)
		return err
	}

	preservedResult, err := tx.ExecContext(
		ctx,
		`UPDATE kycs
		 SET verification_provider = $1
		 WHERE kyc_status IN ($2, $3)`,
		model.VerificationProviderSumsub,
		model.StatusApproved,
		model.StatusFinalRejected,
	)
	if err != nil {
		return fmt.Errorf("backfill preserved Sumsub provider: %w", err)
	}
	preservedRows, err := preservedResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("read preserved update count: %w", err)
	}
	if preservedRows != before.Preserved {
		return fmt.Errorf("preserved update affected %d rows, expected %d", preservedRows, before.Preserved)
	}

	resetResult, err := tx.ExecContext(
		ctx,
		`UPDATE kycs
		 SET kyc_status = $1,
		     applicant_id = '',
		     applicant_type = '',
		     country = '',
		     vies_registered = FALSE,
		     verification_provider = $2
		 WHERE kyc_status NOT IN ($3, $4)`,
		model.StatusAccountCreated,
		model.VerificationProviderDidit,
		model.StatusApproved,
		model.StatusFinalRejected,
	)
	if err != nil {
		return fmt.Errorf("reset non-preserved KYC rows: %w", err)
	}
	resetRows, err := resetResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("read reset update count: %w", err)
	}
	if resetRows != before.Reset {
		return fmt.Errorf("reset update affected %d rows, expected %d", resetRows, before.Reset)
	}

	resetUserInfosResult, err := tx.ExecContext(
		ctx,
		`DELETE FROM user_infos ui
		 WHERE EXISTS (
		   SELECT 1
		   FROM kycs k
		   LEFT JOIN accounts a ON a.email = k.email
		   WHERE k.verification_provider = $1
		     AND (
		       LOWER(BTRIM(ui.email)) = LOWER(BTRIM(k.email))
		       OR (a.address IS NOT NULL AND ui.blockchain_address = a.address)
		     )
		 )`,
		model.VerificationProviderDidit,
	)
	if err != nil {
		return fmt.Errorf("delete reset-cohort UserInfo rows: %w", err)
	}
	resetUserInfos, err := resetUserInfosResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("read reset UserInfo delete count: %w", err)
	}
	if resetUserInfos != before.ResetUserInfos {
		return fmt.Errorf(
			"reset UserInfo delete affected %d rows, expected %d",
			resetUserInfos,
			before.ResetUserInfos,
		)
	}

	after, err := collectCutoverAggregate(ctx, tx)
	if err != nil {
		return err
	}
	if err = validatePostCutoverAggregate(before, after); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit cutover transaction: %w", err)
	}

	printAggregate(output, "before", before)
	printAggregate(output, "after", after)
	fmt.Fprintln(output, "result=applied_and_committed")
	return nil
}

func collectCutoverAggregate(ctx context.Context, q queryer) (cutoverAggregate, error) {
	aggregate := cutoverAggregate{
		Statuses:  make(map[string]int64),
		Providers: make(map[string]int64),
	}

	err := q.QueryRowContext(
		ctx,
		`SELECT
		   COUNT(*),
		   COUNT(*) FILTER (WHERE kyc_status = $1),
		   COUNT(*) FILTER (WHERE kyc_status = $2),
		   COUNT(*) FILTER (WHERE kyc_status IN ($1, $2)),
		   COUNT(*) FILTER (WHERE kyc_status NOT IN ($1, $2)),
		   COUNT(*) FILTER (
		     WHERE kyc_status NOT IN ($1, $2)
		       AND (
		         COALESCE(applicant_id, '') <> ''
		         OR COALESCE(applicant_type, '') <> ''
		         OR COALESCE(country, '') <> ''
		         OR vies_registered
		       )
		   ),
		   COUNT(*) FILTER (
		     WHERE uuid IS NULL
		        OR BTRIM(CAST(uuid AS text)) = ''
		        OR email IS NULL
		        OR BTRIM(email) = ''
		        OR BTRIM(email) <> email
		   )
		 FROM kycs`,
		model.StatusApproved,
		model.StatusFinalRejected,
	).Scan(
		&aggregate.Total,
		&aggregate.Approved,
		&aggregate.FinalRejected,
		&aggregate.Preserved,
		&aggregate.Reset,
		&aggregate.ResetRowsWithLegacyData,
		&aggregate.InvalidIdentityRows,
	)
	if err != nil {
		return cutoverAggregate{}, fmt.Errorf("read KYC aggregate: %w", err)
	}

	if err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_infos`).Scan(&aggregate.UserInfos); err != nil {
		return cutoverAggregate{}, fmt.Errorf("read UserInfo aggregate: %w", err)
	}
	if err = q.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM user_infos ui
		 WHERE EXISTS (
		   SELECT 1
		   FROM kycs k
		   LEFT JOIN accounts a ON a.email = k.email
		   WHERE k.kyc_status NOT IN ($1, $2)
		     AND (
		       LOWER(BTRIM(ui.email)) = LOWER(BTRIM(k.email))
		       OR (a.address IS NOT NULL AND ui.blockchain_address = a.address)
		     )
		 )`,
		model.StatusApproved,
		model.StatusFinalRejected,
	).Scan(&aggregate.ResetUserInfos); err != nil {
		return cutoverAggregate{}, fmt.Errorf("read reset-cohort UserInfo aggregate: %w", err)
	}
	if err = q.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM kycs k
		 WHERE k.kyc_status = $1
		   AND NOT EXISTS (
		     SELECT 1
		     FROM user_infos ui
		     LEFT JOIN accounts a ON a.address = ui.blockchain_address
		     WHERE LOWER(BTRIM(ui.email)) = LOWER(BTRIM(k.email))
		        OR LOWER(BTRIM(a.email)) = LOWER(BTRIM(k.email))
		   )`,
		model.StatusApproved,
	).Scan(&aggregate.ApprovedMissingUserInfos); err != nil {
		return cutoverAggregate{}, fmt.Errorf("read approved missing UserInfo aggregate: %w", err)
	}
	if err = q.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM (
		   SELECT uuid FROM kycs GROUP BY uuid HAVING COUNT(*) > 1
		 ) duplicate_uuids`,
	).Scan(&aggregate.DuplicateUuidGroups); err != nil {
		return cutoverAggregate{}, fmt.Errorf("read duplicate UUID aggregate: %w", err)
	}
	if err = q.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM (
		   SELECT LOWER(BTRIM(email))
		   FROM kycs
		   WHERE email IS NOT NULL
		   GROUP BY LOWER(BTRIM(email))
		   HAVING COUNT(*) > 1
		 ) duplicate_emails`,
	).Scan(&aggregate.DuplicateEmailGroups); err != nil {
		return cutoverAggregate{}, fmt.Errorf("read duplicate email aggregate: %w", err)
	}

	statusRows, err := q.QueryContext(ctx, `SELECT kyc_status, COUNT(*) FROM kycs GROUP BY kyc_status ORDER BY kyc_status`)
	if err != nil {
		return cutoverAggregate{}, fmt.Errorf("read status aggregates: %w", err)
	}
	for statusRows.Next() {
		var status string
		var count int64
		if err = statusRows.Scan(&status, &count); err != nil {
			statusRows.Close()
			return cutoverAggregate{}, fmt.Errorf("scan status aggregate: %w", err)
		}
		aggregate.Statuses[status] = count
	}
	if err = statusRows.Err(); err != nil {
		statusRows.Close()
		return cutoverAggregate{}, fmt.Errorf("iterate status aggregates: %w", err)
	}
	if err = statusRows.Close(); err != nil {
		return cutoverAggregate{}, fmt.Errorf("close status aggregates: %w", err)
	}

	providerRows, err := q.QueryContext(
		ctx,
		`SELECT COALESCE(verification_provider, ''), COUNT(*)
		 FROM kycs
		 GROUP BY COALESCE(verification_provider, '')
		 ORDER BY COALESCE(verification_provider, '')`,
	)
	if err != nil {
		return cutoverAggregate{}, fmt.Errorf("read provider aggregates: %w", err)
	}
	for providerRows.Next() {
		var provider string
		var count int64
		if err = providerRows.Scan(&provider, &count); err != nil {
			providerRows.Close()
			return cutoverAggregate{}, fmt.Errorf("scan provider aggregate: %w", err)
		}
		aggregate.Providers[provider] = count
	}
	if err = providerRows.Err(); err != nil {
		providerRows.Close()
		return cutoverAggregate{}, fmt.Errorf("iterate provider aggregates: %w", err)
	}
	if err = providerRows.Close(); err != nil {
		return cutoverAggregate{}, fmt.Errorf("close provider aggregates: %w", err)
	}

	return aggregate, nil
}

func validatePreCutoverAggregate(aggregate cutoverAggregate, opts cutoverOptions) error {
	if aggregate.Preserved+aggregate.Reset != aggregate.Total {
		return errors.New("preserved plus reset count does not equal total")
	}
	if aggregate.Approved+aggregate.FinalRejected != aggregate.Preserved {
		return errors.New("approved plus finalRejected count does not equal preserved")
	}
	if aggregate.DuplicateUuidGroups != 0 || aggregate.DuplicateEmailGroups != 0 {
		return fmt.Errorf(
			"duplicate identities found: uuid_groups=%d email_groups=%d",
			aggregate.DuplicateUuidGroups,
			aggregate.DuplicateEmailGroups,
		)
	}
	if aggregate.InvalidIdentityRows != 0 {
		return fmt.Errorf("invalid UUID/email identities found: rows=%d", aggregate.InvalidIdentityRows)
	}
	if aggregate.ApprovedMissingUserInfos != 0 {
		return fmt.Errorf(
			"approved KYC rows missing UserInfo: rows=%d",
			aggregate.ApprovedMissingUserInfos,
		)
	}

	allowedStatuses := map[string]struct{}{
		model.StatusAccountCreated: {},
		model.StatusInit:           {},
		model.StatusPending:        {},
		model.StatusPrechecked:     {},
		model.StatusQueued:         {},
		model.StatusCompleted:      {},
		model.StatusApproved:       {},
		model.StatusOnHold:         {},
		model.StatusRejected:       {},
		model.StatusFinalRejected:  {},
	}
	for status := range aggregate.Statuses {
		if _, allowed := allowedStatuses[status]; !allowed {
			return fmt.Errorf("unexpected KYC status %q", status)
		}
	}
	for provider := range aggregate.Providers {
		if provider != "" && provider != model.VerificationProviderSumsub {
			return fmt.Errorf(
				"unexpected pre-cutover verification provider %q; refuse to run after Didit traffic starts",
				provider,
			)
		}
	}

	if opts.ExpectedTotal >= 0 &&
		(opts.ExpectedTotal != aggregate.Total ||
			opts.ExpectedPreserved != aggregate.Preserved ||
			opts.ExpectedReset != aggregate.Reset ||
			opts.ExpectedResetUserInfos != aggregate.ResetUserInfos) {
		return fmt.Errorf(
			"expected counts do not match: expected total=%d preserved=%d reset=%d reset_user_infos=%d; actual total=%d preserved=%d reset=%d reset_user_infos=%d",
			opts.ExpectedTotal,
			opts.ExpectedPreserved,
			opts.ExpectedReset,
			opts.ExpectedResetUserInfos,
			aggregate.Total,
			aggregate.Preserved,
			aggregate.Reset,
			aggregate.ResetUserInfos,
		)
	}
	return nil
}

func validatePostCutoverAggregate(before, after cutoverAggregate) error {
	if after.Total != before.Total {
		return fmt.Errorf("KYC total changed from %d to %d", before.Total, after.Total)
	}
	if after.UserInfos != before.UserInfos-before.ResetUserInfos {
		return fmt.Errorf(
			"UserInfo total changed from %d to %d; expected removal of %d reset-cohort rows",
			before.UserInfos,
			after.UserInfos,
			before.ResetUserInfos,
		)
	}
	if after.ResetUserInfos != 0 {
		return fmt.Errorf("reset-cohort UserInfo rows remain after cutover: %d", after.ResetUserInfos)
	}
	if after.ApprovedMissingUserInfos != 0 {
		return fmt.Errorf(
			"approved KYC rows missing UserInfo after cutover: %d",
			after.ApprovedMissingUserInfos,
		)
	}
	if after.Approved != before.Approved || after.FinalRejected != before.FinalRejected {
		return errors.New("preserved approved/finalRejected totals changed")
	}
	if after.Statuses[model.StatusAccountCreated] != before.Reset {
		return fmt.Errorf(
			"post-cutover accCreated count is %d, expected %d",
			after.Statuses[model.StatusAccountCreated],
			before.Reset,
		)
	}
	for status, count := range after.Statuses {
		if status != model.StatusAccountCreated &&
			status != model.StatusApproved &&
			status != model.StatusFinalRejected &&
			count != 0 {
			return fmt.Errorf("post-cutover status %q still has %d rows", status, count)
		}
	}
	if after.Providers[model.VerificationProviderSumsub] != before.Preserved {
		return fmt.Errorf(
			"post-cutover Sumsub provider count is %d, expected %d",
			after.Providers[model.VerificationProviderSumsub],
			before.Preserved,
		)
	}
	if after.Providers[model.VerificationProviderDidit] != before.Reset {
		return fmt.Errorf(
			"post-cutover Didit provider count is %d, expected %d",
			after.Providers[model.VerificationProviderDidit],
			before.Reset,
		)
	}
	if len(after.Providers) != expectedProviderGroupCount(before) {
		return fmt.Errorf("unexpected post-cutover provider groups: %v", sortedKeys(after.Providers))
	}
	if after.ResetRowsWithLegacyData != 0 {
		return fmt.Errorf("post-cutover reset rows still contain legacy fields: %d", after.ResetRowsWithLegacyData)
	}
	return nil
}

func expectedProviderGroupCount(before cutoverAggregate) int {
	count := 0
	if before.Preserved > 0 {
		count++
	}
	if before.Reset > 0 {
		count++
	}
	return count
}

func printAggregate(output io.Writer, label string, aggregate cutoverAggregate) {
	fmt.Fprintf(output, "aggregate=%s\n", label)
	fmt.Fprintf(output, "total=%d\n", aggregate.Total)
	fmt.Fprintf(output, "approved=%d\n", aggregate.Approved)
	fmt.Fprintf(output, "final_rejected=%d\n", aggregate.FinalRejected)
	fmt.Fprintf(output, "preserved=%d\n", aggregate.Preserved)
	fmt.Fprintf(output, "reset=%d\n", aggregate.Reset)
	fmt.Fprintf(output, "user_infos=%d\n", aggregate.UserInfos)
	fmt.Fprintf(output, "reset_user_infos=%d\n", aggregate.ResetUserInfos)
	fmt.Fprintf(output, "approved_missing_user_infos=%d\n", aggregate.ApprovedMissingUserInfos)
	fmt.Fprintf(output, "duplicate_uuid_groups=%d\n", aggregate.DuplicateUuidGroups)
	fmt.Fprintf(output, "duplicate_email_groups=%d\n", aggregate.DuplicateEmailGroups)
	fmt.Fprintf(output, "invalid_identity_rows=%d\n", aggregate.InvalidIdentityRows)
	fmt.Fprintf(output, "reset_rows_with_legacy_data=%d\n", aggregate.ResetRowsWithLegacyData)
	for _, status := range sortedKeys(aggregate.Statuses) {
		fmt.Fprintf(output, "status[%s]=%d\n", status, aggregate.Statuses[status])
	}
	for _, provider := range sortedKeys(aggregate.Providers) {
		display := provider
		if display == "" {
			display = "unassigned"
		}
		fmt.Fprintf(output, "provider[%s]=%d\n", display, aggregate.Providers[provider])
	}
}

func sortedKeys(values map[string]int64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
