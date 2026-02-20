package storage

import "gorm.io/gorm"

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
