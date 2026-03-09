package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func GetLatestInvoiceBlock() (*int64, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var invoice model.InvoiceClient
	txRead := db.Order("block_number DESC").First(&invoice, "block_number IS NOT NULL")
	if txRead.Error != nil {
		if txRead.Error == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return invoice.BlockNumber, true, nil
}

func GetInvoiceByID(id string) (*model.InvoiceClient, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var invoice model.InvoiceClient
	txRead := db.Find(&invoice, "uuid = ?", id)
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, false, nil
	}

	return &invoice, true, nil
}

func CreateInvoice(invoice *model.InvoiceClient) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	// Persist explicit zero-values (e.g. IsUe=false) instead of letting DB defaults override them.
	txCreate := db.Select("*").Create(invoice)
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

func UpdateInvoice(invoice *model.InvoiceClient) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	// Keep update behavior consistent and include zero-values when saving.
	txUpdate := db.Select("*").Save(invoice)
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

func GetUserInvoices(address string) (*[]model.InvoiceClient, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var invoices []model.InvoiceClient
	txRead := db.Find(&invoices, "address = ? && status =  paid", address)
	if txRead.Error != nil {
		return nil, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, nil
	}

	return &invoices, nil
}
