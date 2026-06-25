package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func CreateOrUpdateUserInfo(userInfo *model.UserInfo) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txUpdate := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "blockchain_address"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"email",
			"name",
			"surname",
			"company_name",
			"identification_code",
			"address",
			"state",
			"city",
			"country",
			"is_company",
		}),
	}).Create(userInfo)
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

func GetUserInfoByAddress(address string) (*model.UserInfo, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var userInfo model.UserInfo
	txRead := db.Find(&userInfo, "blockchain_address = ?", address)
	if txRead.Error != nil {
		return nil, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, nil
	}

	return &userInfo, nil
}
