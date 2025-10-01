package storage

import (
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func CreateBurnEvent(burnEvent *model.BurnEvent) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&burnEvent)
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

func GetBurnEventsByOwnerAddress(userAddress string) ([]model.BurnEvent, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var bEvent []model.BurnEvent
	txRead := db.Preload("CspProfile").Where("csp_owner =  ? ", userAddress).Order("block_number DESC").Find(&bEvent)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	return bEvent, nil
}

func GetBurnEventsForUserInTimeRange(start, end time.Time, userAddress string) ([]model.BurnEvent, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var bEvent []model.BurnEvent
	txRead := db.Preload("CspProfile").Where("burn_timestamp >= ? AND burn_timestamp <= ? AND csp_owner = ?", start, end, userAddress).Order("block_number DESC").Find(&bEvent)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	return bEvent, nil
}
