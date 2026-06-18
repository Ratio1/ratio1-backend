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

	return pInvs, nil
}

func GetDraftByReportId(id, userAddress string) (*model.InvoiceDraft, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var pInvs model.InvoiceDraft
	txRead := db.Preload("CspProfile").Preload("UserProfile").Where("draft_id =  ? AND user_address = ? ", id, userAddress).Find(&pInvs)
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
	txRead := db.Preload("CspProfile").Preload("UserProfile").Where("draft_id =  ? AND csp_owner = ? ", id, userAddress).Find(&pInvs)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	return &pInvs, nil
}

func CreateInvoiceDraft(tx *gorm.DB, pInv *model.InvoiceDraft) error {
	exec, err := getExecutor(tx)
	if err != nil {
		return err
	}

	txCreate := exec.Create(pInv)
	if txCreate.Error != nil {
		return txCreate.Error
	}
	if txCreate.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func UpdateInvoiceDraft(tx *gorm.DB, pInv *model.InvoiceDraft) error {
	exec, err := getExecutor(tx)
	if err != nil {
		return err
	}

	txUpdate := exec.Save(pInv)
	if txUpdate.Error != nil {
		return txUpdate.Error
	}
	if txUpdate.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
