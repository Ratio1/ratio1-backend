package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/stretchr/testify/require"
)

func TestGetAllocationsByJobIDsForJobDetails(t *testing.T) {
	db, err := GetDB()
	require.NoError(t, err)

	now := time.Now().UTC()
	suffix := now.UnixNano()
	jobID1 := fmt.Sprintf("job-details-%d-1", suffix)
	jobID2 := fmt.Sprintf("job-details-%d-2", suffix)
	missingJobID := fmt.Sprintf("job-details-%d-missing", suffix)
	jobIDs := []string{jobID1, jobID2, missingJobID}

	t.Cleanup(func() {
		require.NoError(t, db.Where("job_id IN ?", jobIDs).Delete(&model.Allocation{}).Error)
	})

	allocations := []model.Allocation{
		allocationForJobDetailsTest(jobID1, "old job", model.JobType(1), "old project", 10, now.Add(-2*time.Hour)),
		allocationForJobDetailsTest(jobID1, "latest job", model.JobType(2), "latest project", 11, now.Add(-time.Hour)),
		allocationForJobDetailsTest(jobID1, "", model.JobType(3), "ignored project", 12, now),
		allocationForJobDetailsTest(jobID2, "same block older", model.JobType(4), "same block old project", 20, now.Add(-30*time.Minute)),
		allocationForJobDetailsTest(jobID2, "same block newer", model.JobType(5), "same block new project", 20, now.Add(-20*time.Minute)),
	}
	require.NoError(t, db.Create(&allocations).Error)

	result, err := GetAllocationsByJobIDsForJobDetails(jobIDs)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.NotContains(t, result, missingJobID)

	require.Equal(t, "latest job", result[jobID1].JobName)
	require.Equal(t, model.JobType(2), result[jobID1].JobType)
	require.Equal(t, "latest project", result[jobID1].ProjectName)

	require.Equal(t, "same block newer", result[jobID2].JobName)
	require.Equal(t, model.JobType(5), result[jobID2].JobType)
	require.Equal(t, "same block new project", result[jobID2].ProjectName)
}

func TestGetAllocationsByJobIDsForJobDetailsEmptyInput(t *testing.T) {
	result, err := GetAllocationsByJobIDsForJobDetails(nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func allocationForJobDetailsTest(jobID, jobName string, jobType model.JobType, projectName string, blockNumber int64, allocationCreation time.Time) model.Allocation {
	return model.Allocation{
		AllocationCreation: allocationCreation,
		BlockNumber:        blockNumber,
		TxHash:             "0x0000000000000000000000000000000000000000000000000000000000000000",
		JobId:              jobID,
		JobName:            jobName,
		JobType:            jobType,
		ProjectName:        projectName,
		NodeAddress:        "0x0000000000000000000000000000000000000000",
		UserAddress:        "0x0000000000000000000000000000000000000000",
		CspAddress:         "0x0000000000000000000000000000000000000000",
		CspOwner:           "0x0000000000000000000000000000000000000000",
		UsdcAmountPayed:    "0",
	}
}
