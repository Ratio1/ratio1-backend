package storage

import (
	"errors"

	"gorm.io/gorm"
)

func WithTransaction(fn func(tx *gorm.DB) error) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	return db.Transaction(fn)
}

func getExecutor(tx *gorm.DB) (*gorm.DB, error) {
	if tx != nil {
		return tx, nil
	}
	return GetDB()
}

func LockTransaction(tx *gorm.DB, lockName string) error {
	if tx == nil {
		return errors.New("transaction is required")
	}

	return tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", lockName).Error
}
