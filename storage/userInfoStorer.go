package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func CreateUserInfo(userInfo *model.UserInfo) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&userInfo)
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

func UpdateUserInfo(userInfo *model.UserInfo) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txUpdate := db.Save(&userInfo)
	if txUpdate.Error != nil {
		txUpdate.Rollback()
		return txUpdate.Error
	}
	if txUpdate.RowsAffected == 0 {
		txUpdate.Rollback()
		return gorm.ErrRecordNotFound
	}

	return nil
}
