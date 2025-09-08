package storage

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func GetLatestAllocationBlock() (int64, error) {
	db, err := GetDB()
	if err != nil {
		return 0, err
	}

	var allocation model.Allocation
	txRead := db.Order("block_number DESC").First(&allocation)
	if txRead.Error != nil {
		if txRead.Error == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}

	return allocation.BlockNumber, nil
}

func CreateAllocation(alloc *model.Allocation) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&alloc)
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

func UpdateAllocation(alloc *model.Allocation) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txUpdate := db.Save(&alloc)
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

func GetAllocationsByCspAndUser(cspAddress, userAddress, nodeAddress string) ([]model.Allocation, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var allocations []model.Allocation
	txRead := db.Where("csp_address = ? AND user_address = ? AND node_address = ?", cspAddress, userAddress, nodeAddress).Find(&allocations)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	if len(allocations) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return allocations, nil
}

func GetUnClaimedAllocations() ([]model.Allocation, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var allocations []model.Allocation
	txRead := db.
		Where("invoice_id IS NULL").
		Preload("CspProfile").
		Preload("UserProfile").
		Find(&allocations)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	if len(allocations) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return allocations, nil
}

func GetAllocationsByDraftId(draftId string) ([]model.Allocation, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var allocations []model.Allocation
	txRead := db.Where("draft_id = ? ", draftId).Find(&allocations)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	if len(allocations) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return allocations, nil
}
