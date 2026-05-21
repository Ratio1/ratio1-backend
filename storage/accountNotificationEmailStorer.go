package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
)

func GetAccountNotificationEmailByAddress(address string) (*model.AccountNotificationEmail, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var notificationEmail model.AccountNotificationEmail
	txRead := db.Find(&notificationEmail, "account_address = ?", address)
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, false, nil
	}

	return &notificationEmail, true, nil
}

func CreateOrUpdateAccountNotificationEmail(notificationEmail *model.AccountNotificationEmail) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txUpdate := db.Save(notificationEmail)
	if txUpdate.Error != nil {
		txUpdate.Rollback()
		return txUpdate.Error
	}
	if txUpdate.RowsAffected == 0 {
		txUpdate.Rollback()
		return nil
	}

	return nil
}

func DeleteAccountNotificationEmail(address string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txDelete := db.Delete(&model.AccountNotificationEmail{}, "account_address = ?", address)
	if txDelete.Error != nil {
		txDelete.Rollback()
		return txDelete.Error
	}

	return nil
}
