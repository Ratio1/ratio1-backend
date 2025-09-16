package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func GetDraftListByNodeOwner(userAddress string) ([]model.InvoiceDraft, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var pInvs []model.InvoiceDraft
	txRead := db.Preload("CspProfile").Preload("UserProfile").Where("user_address =  ? ", userAddress).Find(&pInvs)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	if len(pInvs) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return pInvs, nil
}

func GetDraftListByCSP(userAddress string) ([]model.InvoiceDraft, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var pInvs []model.InvoiceDraft
	txRead := db.Preload("CspProfile").Preload("UserProfile").Where("csp_owner =  ? ", userAddress).Find(&pInvs)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	if len(pInvs) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return pInvs, nil
}

func GetDraftByReportId(id, userAddress string) (*model.InvoiceDraft, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var pInvs model.InvoiceDraft
	txRead := db.Preload("CspProfile").Preload("UserProfile").Where("invoice_id =  ? AND user_address = ? ", id, userAddress).Find(&pInvs)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	return &pInvs, nil
}

func GetCspDraftByReportId(id, userAddress string) (*model.InvoiceDraft, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var pInvs model.InvoiceDraft
	txRead := db.Preload("CspProfile").Preload("UserProfile").Where("invoice_id =  ? AND csp_owner = ? ", id, userAddress).Find(&pInvs)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	return &pInvs, nil
}

func CreateInvoiceDraft(pInv *model.InvoiceDraft) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&pInv)
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

func UpdateInvoiceDraft(pInv *model.InvoiceDraft) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txUpdate := db.Save(&pInv)
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
