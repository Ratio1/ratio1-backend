package storage

import (
	"fmt"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
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

func CreateAllocation(tx *gorm.DB, alloc *model.Allocation) error {
	exec, err := getExecutor(tx)
	if err != nil {
		return err
	}

	txCreate := exec.Create(alloc)
	if txCreate.Error != nil {
		return txCreate.Error
	}
	if txCreate.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func UpdateAllocation(tx *gorm.DB, alloc *model.Allocation) error {
	exec, err := getExecutor(tx)
	if err != nil {
		return err
	}

	txUpdate := exec.Save(alloc)
	if txUpdate.Error != nil {
		return txUpdate.Error
	}
	if txUpdate.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func ClaimAllocationsForDraft(tx *gorm.DB, allocations []model.Allocation, draftID uuid.UUID) error {
	if len(allocations) == 0 {
		return nil
	}

	exec, err := getExecutor(tx)
	if err != nil {
		return err
	}

	ids := make([]uint, 0, len(allocations))
	for _, alloc := range allocations {
		ids = append(ids, alloc.Id)
	}

	txUpdate := exec.Model(&model.Allocation{}).
		Where("id IN ? AND draft_id IS NULL", ids).
		Update("draft_id", draftID)
	if txUpdate.Error != nil {
		return txUpdate.Error
	}
	if txUpdate.RowsAffected != int64(len(ids)) {
		return fmt.Errorf("claimed %d of %d allocations", txUpdate.RowsAffected, len(ids))
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

func GetMonthlyUnclaimedAllocations(now time.Time) ([]model.Allocation, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	currStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevStart := currStart.AddDate(0, -1, 0)

	var allocations []model.Allocation
	tx := db.
		Where("allocation_creation >= ? AND allocation_creation < ?", prevStart, currStart).
		Where("draft_id IS NULL").
		Preload("CspProfile").
		Preload("UserProfile").
		Find(&allocations)
	if tx.Error != nil {
		return nil, tx.Error
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

func GetAllocationByJobIDForJobDetails(jobId string) (*model.Allocation, error) {
	allocations, err := GetAllocationsByJobIDsForJobDetails([]string{jobId})
	if err != nil {
		return nil, err
	}

	allocation, ok := allocations[jobId]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}

	return allocation, nil
}

func GetAllocationsByJobIDsForJobDetails(jobIDs []string) (map[string]*model.Allocation, error) {
	allocationsByJobID := make(map[string]*model.Allocation)
	if len(jobIDs) == 0 {
		return allocationsByJobID, nil
	}

	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var allocations []model.Allocation
	txRead := db.
		Where("job_id IN ? AND job_name IS NOT NULL AND job_name <> ''", jobIDs).
		Order("job_id, block_number DESC, allocation_creation DESC, id DESC").
		Distinct("ON (job_id) *").
		Find(&allocations)
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	for i := range allocations {
		allocationsByJobID[allocations[i].JobId] = &allocations[i]
	}

	return allocationsByJobID, nil
}
