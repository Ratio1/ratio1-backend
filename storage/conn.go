package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

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

func Connect() {
	once.Do(func() {
		sqlDb, err := sql.Open("postgres", config.Config.Database.Url())
		if err != nil {
			panic(err)
		}
		sqlDb.SetMaxOpenConns(config.Config.Database.MaxOpenConns)
		sqlDb.SetMaxIdleConns(config.Config.Database.MaxIdleConns)
		conn, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDb}))
		if err != nil {
			panic(err)
		}

		database = conn

		err = TryMigrate()
		if err != nil {
			panic(err)
		}
	})
}

func TryMigrate() error {
	if err := validateExistingKycEmailsAreUnique(database); err != nil {
		return err
	}

	err := database.AutoMigrate(
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
		&model.VerificationNotification{},
	)
	if err != nil {
		return err
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

func GetDB() (*gorm.DB, error) {
	if database == nil {
		return nil, NoDBError
	}

	return database, nil
}
