package storage

import (
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
