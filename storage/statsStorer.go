package storage

import (
	"errors"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func CreateStats(stats *model.Stats) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&stats)
	if txCreate.Error != nil {
		txCreate.Rollback()
		return txCreate.Error
	}
	if txCreate.RowsAffected == 0 {
		txCreate.Rollback()
		return gorm.ErrRecordNotFound
	}

	return nil
}

func GetLatestStats() (*model.Stats, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var stats model.Stats
	tx := db.Order("creation_timestamp DESC").First(&stats)

	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}

	return &stats, nil
}

func GetAllStatsASC() (*[]model.Stats, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var stats []model.Stats
	tx := db.Order("creation_timestamp ASC").Find(&stats)

	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}

	return &stats, nil
}
