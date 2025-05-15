package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func CreateSeller(seller *model.Seller) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&seller)
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

func GetSellerCodeByAddress(address string) (*string, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var sel model.Seller
	txRead := db.Find(&sel, "account_id = ?", address)
	if txRead.Error != nil {
		return nil, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, nil
	}
	return &sel.SellerCode, nil
}

func AddressHasCode(accountID string) (bool, error) {
	db, err := GetDB()
	if err != nil {
		return false, err
	}

	var sel model.Seller
	txRead := db.Find(&sel, "account_id = ?", accountID)
	if txRead.Error != nil {
		return false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return false, nil
	}
	return true, nil
}

func SellerCodeDoExist(sellerCode string) (bool, error) {
	db, err := GetDB()
	if err != nil {
		return false, err
	}

	var sel model.Seller
	txRead := db.Find(&sel, "seller_code = ?", sellerCode)
	if txRead.Error != nil {
		return false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return false, nil
	}
	return true, nil
}
