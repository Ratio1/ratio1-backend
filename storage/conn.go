package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	once     sync.Once
	database *gorm.DB

	NoDBError = errors.New("no DB Connection")
)

const (
	migrationMaxOpenConns   = 2
	migrationMaxIdleConns   = 1
	migrationConnectTimeout = 10 * time.Second
)

func Connect() {
	once.Do(func() {
		conn, _, err := openDatabase(
			config.Config.Database.Url(),
			config.Config.Database.MaxOpenConns,
			config.Config.Database.MaxIdleConns,
		)
		if err != nil {
			panic(err)
		}

		database = conn
	})
}

func Migrate(ctx context.Context, databaseConfig config.DatabaseConfig) error {
	serverVersion, err := getDatabaseServerVersion(ctx, databaseConfig.Url())
	if err != nil {
		return err
	}

	databaseURL := migrationDatabaseURL(databaseConfig, serverVersion)
	conn, sqlDB, err := openDatabase(databaseURL, migrationMaxOpenConns, migrationMaxIdleConns)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	return migrateDatabase(ctx, conn)
}

func migrateDatabase(ctx context.Context, conn *gorm.DB) error {
	if err := validateExistingKycEmailsAreUnique(conn.WithContext(ctx)); err != nil {
		return err
	}

	if err := conn.WithContext(ctx).AutoMigrate(
		&model.Account{},
		&model.AccountNotificationEmail{},
		&model.Kyc{},
		&model.InvoiceClient{},
		&model.Seller{},
		&model.Stats{},
		&model.Allocation{},
		&model.Preference{},
		&model.InvoiceDraft{},
		&model.UserInfo{},
		&model.BurnEvent{},
		&model.Branding{},
		&model.VerificationSession{},
		&model.VerificationWebhookEvent{},
	); err != nil {
		return err
	}

	return verifyMigrationSchema(ctx, conn)
}

var requiredMigrationTables = []string{
	"account_notification_emails",
	"accounts",
	"allocations",
	"brandings",
	"burn_events",
	"invoice_clients",
	"invoice_drafts",
	"kycs",
	"preferences",
	"sellers",
	"stats",
	"user_infos",
	"verification_sessions",
	"verification_webhook_events",
}

var requiredMigrationIndexes = map[string]struct {
	table         string
	keyDefinition string
	predicate     string
	unique        bool
}{
	"idx_allocations_draft_creation": {
		table:         "allocations",
		keyDefinition: "using btree (draft_id, allocation_creation)",
	},
	"idx_allocations_job_latest": {
		table:         "allocations",
		keyDefinition: "using btree (job_id, block_number desc, allocation_creation desc, id desc)",
		predicate:     "job_name is not null and job_name != ''",
	},
	"idx_burn_events_owner_block": {
		table:         "burn_events",
		keyDefinition: "using btree (csp_owner, block_number desc)",
	},
	"idx_invoice_clients_block": {
		table:         "invoice_clients",
		keyDefinition: "using btree (block_number desc)",
	},
	"idx_kycs_email": {
		table:         "kycs",
		keyDefinition: "using btree (email)",
		unique:        true,
	},
}

func verifyMigrationSchema(ctx context.Context, conn *gorm.DB) error {
	var tables []string
	if err := conn.WithContext(ctx).Raw(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_type = 'BASE TABLE' AND table_name IN ?",
		requiredMigrationTables,
	).Scan(&tables).Error; err != nil {
		return fmt.Errorf("verify migrated tables: %w", err)
	}

	presentTables := make(map[string]bool, len(tables))
	for _, table := range tables {
		presentTables[table] = true
	}
	for _, table := range requiredMigrationTables {
		if !presentTables[table] {
			return fmt.Errorf("verify migrated schema: missing table %s", table)
		}
	}

	type indexDefinition struct {
		TableName  string `gorm:"column:tablename"`
		IndexName  string `gorm:"column:indexname"`
		Definition string `gorm:"column:indexdef"`
	}
	indexNames := make([]string, 0, len(requiredMigrationIndexes))
	for indexName := range requiredMigrationIndexes {
		indexNames = append(indexNames, indexName)
	}
	var indexes []indexDefinition
	if err := conn.WithContext(ctx).Raw(
		"SELECT tablename, indexname, indexdef FROM pg_indexes WHERE schemaname = CURRENT_SCHEMA() AND indexname IN ?",
		indexNames,
	).Scan(&indexes).Error; err != nil {
		return fmt.Errorf("verify migrated indexes: %w", err)
	}

	presentIndexes := make(map[string]indexDefinition, len(indexes))
	for _, index := range indexes {
		presentIndexes[migrationIndexKey(index.TableName, index.IndexName)] = index
	}
	for indexName, required := range requiredMigrationIndexes {
		index, ok := presentIndexes[migrationIndexKey(required.table, indexName)]
		if !ok {
			return fmt.Errorf("verify migrated schema: missing index %s on table %s", indexName, required.table)
		}
		if err := verifyMigrationIndexDefinition(
			indexName,
			index.Definition,
			required.keyDefinition,
			required.predicate,
			required.unique,
		); err != nil {
			return err
		}
	}

	return nil
}

func migrationIndexKey(tableName, indexName string) string {
	return tableName + "\x00" + indexName
}

func verifyMigrationIndexDefinition(indexName, definition, expectedKey, expectedPredicate string, expectedUnique bool) error {
	definition = normalizeIndexDefinition(definition)
	isUnique := strings.HasPrefix(definition, "create unique index ")
	if isUnique && !expectedUnique {
		return fmt.Errorf("verify migrated schema: index %s is unexpectedly unique", indexName)
	}
	if !isUnique && expectedUnique {
		return fmt.Errorf("verify migrated schema: index %s is unexpectedly non-unique", indexName)
	}

	usingPosition := strings.Index(definition, " using btree ")
	if usingPosition == -1 {
		return fmt.Errorf("verify migrated schema: index %s has unexpected key definition", indexName)
	}
	keyAndPredicate := definition[usingPosition+1:]
	parts := strings.SplitN(keyAndPredicate, " where ", 2)
	if parts[0] != expectedKey {
		return fmt.Errorf("verify migrated schema: index %s has unexpected key definition", indexName)
	}

	predicate := ""
	if len(parts) == 2 {
		predicate = normalizeIndexPredicate(parts[1])
	}
	if predicate != expectedPredicate {
		return fmt.Errorf("verify migrated schema: index %s has unexpected predicate", indexName)
	}

	return nil
}

func validateExistingKycEmailsAreUnique(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Kyc{}) {
		return nil
	}

	var duplicateGroups int64
	err := db.Raw(`
		SELECT COUNT(*)
		FROM (
			SELECT email
			FROM kycs
			WHERE email IS NOT NULL
			GROUP BY email
			HAVING COUNT(*) > 1
		) duplicate_emails
	`).Scan(&duplicateGroups).Error
	if err != nil {
		return fmt.Errorf("preflight KYC email uniqueness: %w", err)
	}
	if duplicateGroups > 0 {
		return fmt.Errorf(
			"cannot add the KYC email unique index: found %d duplicate email groups; run a reviewed data migration first",
			duplicateGroups,
		)
	}

	return nil
}

func normalizeIndexDefinition(definition string) string {
	definition = strings.ToLower(definition)
	definition = strings.ReplaceAll(definition, "\"", "")
	definition = strings.ReplaceAll(definition, "::string", "")
	definition = strings.ReplaceAll(definition, "::text", "")
	definition = strings.ReplaceAll(definition, "<>", "!=")
	definition = strings.ReplaceAll(definition, " asc", "")
	definition = strings.ReplaceAll(definition, ",", ", ")

	return strings.Join(strings.Fields(definition), " ")
}

func normalizeIndexPredicate(predicate string) string {
	predicate = strings.ReplaceAll(predicate, "(", "")
	predicate = strings.ReplaceAll(predicate, ")", "")

	return strings.Join(strings.Fields(predicate), " ")
}

func openDatabase(databaseURL string, maxOpenConns, maxIdleConns int) (*gorm.DB, *sql.DB, error) {
	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, nil, err
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)

	conn, err := gorm.Open(postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	}))
	if err != nil {
		sqlDB.Close()
		return nil, nil, err
	}

	return conn, sqlDB, nil
}

func getDatabaseServerVersion(ctx context.Context, databaseURL string) (string, error) {
	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return "", err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(0)

	probeCtx, cancel := context.WithTimeout(ctx, migrationConnectTimeout)
	defer cancel()

	var serverVersion string
	if err := sqlDB.QueryRowContext(probeCtx, "SELECT version()").Scan(&serverVersion); err != nil {
		return "", fmt.Errorf("detect database server: %w", err)
	}

	return serverVersion, nil
}

func migrationDatabaseURL(databaseConfig config.DatabaseConfig, serverVersion string) string {
	options := "-c statement_timeout=120000"
	if strings.Contains(strings.ToLower(serverVersion), "cockroachdb") {
		options = "-c default_int_size=4 " + options
	}

	return fmt.Sprintf(
		"%s connect_timeout=10 application_name=ratio1_backend_migrate options='%s'",
		databaseConfig.Url(),
		options,
	)
}

func GetDB() (*gorm.DB, error) {
	if database == nil {
		return nil, NoDBError
	}

	return database, nil
}
