package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func GetAccountByAddress(address string) (*model.Account, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var ethAccount model.Account
	txRead := db.Find(&ethAccount, "address = ?", address)
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, false, nil
	}

	return &ethAccount, true, nil
}

func GetAccountByEmail(email string) (*model.Account, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var acc model.Account
	txRead := db.Find(&acc, "email = ?", email)
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, false, nil
	}

	return &acc, true, nil
}

func CreateAccount(account *model.Account) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&account)
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

func UpdateAccount(account *model.Account) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txUpdate := db.Save(&account)
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
