package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func SaveBrand(seller *model.Branding) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Save(&seller)
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

func GetBrandByAddress(address string) (*model.Branding, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var b model.Branding
	txRead := db.First(&b, "user_address = ?", address)
	if txRead.Error != nil {
		return nil, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, nil
	}
	return &b, nil
}

//TODO add getBrands with pagination
