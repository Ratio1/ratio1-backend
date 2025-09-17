package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func GetPreferenceByAddress(userAddress string) (*model.Preference, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var pref model.Preference
	txRead := db.Where("user_address =  ? ", userAddress).Find(&pref)
	if txRead.Error != nil {
		return nil, txRead.Error
	} else if txRead.RowsAffected == 0 {
		return nil, nil
	}

	return &pref, nil
}

func CreatePreference(pref *model.Preference) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&pref)
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

func UpdatePreference(pref *model.Preference) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txUpdate := db.Save(&pref)
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
