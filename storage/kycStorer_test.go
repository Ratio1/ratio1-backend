package storage

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateOrUpdateKycPersistsZeroValuesAndPreservesUuid(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	email := fmt.Sprintf("kyc-zero-values-%s@example.com", uuid.NewString())
	t.Cleanup(func() {
		require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
	})

	receiveUpdates := true
	original := &model.Kyc{
		Uuid:           uuid.New(),
		ApplicantId:    "sumsub-applicant",
		ApplicantType:  model.BusinessCustomer,
		Email:          email,
		KycStatus:      model.StatusApproved,
		LastUpdated:    time.Now().UTC().Truncate(time.Microsecond),
		IsActive:       true,
		HasBeenDeleted: true,
		ReceiveUpdates: &receiveUpdates,
		Country:        "ROU",
		ViesRegistered: true,
	}
	require.NoError(t, CreateOrUpdateKyc(original))

	receiveUpdates = false
	update := &model.Kyc{
		Uuid:           original.Uuid,
		Email:          email,
		KycStatus:      model.StatusInit,
		LastUpdated:    time.Time{},
		IsActive:       false,
		HasBeenDeleted: false,
		ReceiveUpdates: &receiveUpdates,
		ViesRegistered: false,
	}
	require.NoError(t, CreateOrUpdateKyc(update))

	stored, found, err := GetKycByEmail(email)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, original.Uuid, stored.Uuid)
	require.Empty(t, stored.ApplicantId)
	require.Empty(t, stored.ApplicantType)
	require.Equal(t, model.StatusInit, stored.KycStatus)
	require.True(t, stored.LastUpdated.IsZero())
	require.False(t, stored.IsActive)
	require.False(t, stored.HasBeenDeleted)
	require.NotNil(t, stored.ReceiveUpdates)
	require.False(t, *stored.ReceiveUpdates)
	require.Empty(t, stored.Country)
	require.False(t, stored.ViesRegistered)
}

func TestCreateOrUpdateKycRejectsIncompleteRecord(t *testing.T) {
	requireStorageTestDatabase(t)

	receiveUpdates := false
	tests := []struct {
		name string
		kyc  *model.Kyc
	}{
		{
			name: "nil uuid",
			kyc: &model.Kyc{
				Email:          "nil-uuid@example.com",
				KycStatus:      model.StatusInit,
				ReceiveUpdates: &receiveUpdates,
			},
		},
		{
			name: "blank email",
			kyc: &model.Kyc{
				Uuid:           uuid.New(),
				Email:          " ",
				KycStatus:      model.StatusInit,
				ReceiveUpdates: &receiveUpdates,
			},
		},
		{
			name: "missing status",
			kyc: &model.Kyc{
				Uuid:           uuid.New(),
				Email:          "missing-status@example.com",
				ReceiveUpdates: &receiveUpdates,
			},
		},
		{
			name: "missing preference",
			kyc: &model.Kyc{
				Uuid:      uuid.New(),
				Email:     "missing-preference@example.com",
				KycStatus: model.StatusInit,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, CreateOrUpdateKyc(test.kyc))
		})
	}
}

func TestCreateOrUpdateKycRejectsEmailReassignmentToDifferentUuid(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	email := fmt.Sprintf("kyc-identity-conflict-%s@example.com", uuid.NewString())
	t.Cleanup(func() {
		require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
	})

	receiveUpdates := false
	approved := &model.Kyc{
		Uuid:           uuid.New(),
		Email:          email,
		KycStatus:      model.StatusApproved,
		IsActive:       true,
		ReceiveUpdates: &receiveUpdates,
	}
	require.NoError(t, CreateOrUpdateKyc(approved))

	reassignment := &model.Kyc{
		Uuid:           uuid.New(),
		Email:          email,
		KycStatus:      model.StatusInit,
		ReceiveUpdates: &receiveUpdates,
	}
	require.ErrorIs(t, CreateOrUpdateKyc(reassignment), ErrKycIdentityConflict)

	stored, found, err := GetKycByEmail(email)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, approved.Uuid, stored.Uuid)
	require.Equal(t, model.StatusApproved, stored.KycStatus)
	require.True(t, stored.IsActive)
}

func TestTryMigrateAddsKycEmailUniqueIndex(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	require.NoError(t, db.Migrator().DropIndex(&model.Kyc{}, "idx_kycs_email"))
	require.False(t, db.Migrator().HasIndex(&model.Kyc{}, "idx_kycs_email"))

	require.NoError(t, db.AutoMigrate(&model.Kyc{}))
	require.True(t, db.Migrator().HasIndex(&model.Kyc{}, "idx_kycs_email"))
}

func TestTryMigrateRejectsLegacyDuplicateKycEmailsBeforeAddingIndex(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	email := fmt.Sprintf("kyc-duplicate-preflight-%s@example.com", uuid.NewString())
	nullEmailUuidOne := uuid.New()
	nullEmailUuidTwo := uuid.New()
	require.NoError(t, db.Migrator().DropIndex(&model.Kyc{}, "idx_kycs_email"))
	t.Cleanup(func() {
		require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
		require.NoError(t, db.Where("uuid IN ?", []uuid.UUID{nullEmailUuidOne, nullEmailUuidTwo}).Delete(&model.Kyc{}).Error)
		require.NoError(t, db.AutoMigrate(&model.Kyc{}))
	})

	receiveUpdates := false
	require.NoError(t, db.Create([]model.Kyc{
		{
			Uuid:           uuid.New(),
			Email:          email,
			KycStatus:      model.StatusApproved,
			ReceiveUpdates: &receiveUpdates,
		},
		{
			Uuid:           uuid.New(),
			Email:          email,
			KycStatus:      model.StatusFinalRejected,
			ReceiveUpdates: &receiveUpdates,
		},
	}).Error)

	err = TryMigrate()
	require.ErrorContains(t, err, "found 1 duplicate email groups")
	require.False(t, db.Migrator().HasIndex(&model.Kyc{}, "idx_kycs_email"))

	require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO kycs (uuid, email) VALUES (?, NULL), (?, NULL)",
		nullEmailUuidOne,
		nullEmailUuidTwo,
	).Error)
	require.NoError(t, validateExistingKycEmailsAreUnique(db))
	require.NoError(t, db.AutoMigrate(&model.Kyc{}))
	require.True(t, db.Migrator().HasIndex(&model.Kyc{}, "idx_kycs_email"))
}

func TestCreateOrUpdateKycConcurrentCreateKeepsOneRow(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	email := fmt.Sprintf("kyc-concurrent-%s@example.com", uuid.NewString())
	t.Cleanup(func() {
		require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error)
	})

	const workers = 16
	var winners atomic.Int32
	var identityConflicts atomic.Int32
	var unexpectedFailures atomic.Int32
	ready := make(chan struct{}, workers)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer waitGroup.Done()
			ready <- struct{}{}
			<-start
			receiveUpdates := false
			err := CreateOrUpdateKyc(&model.Kyc{
				Uuid:           uuid.New(),
				Email:          email,
				KycStatus:      model.StatusInit,
				ReceiveUpdates: &receiveUpdates,
			})
			switch {
			case err == nil:
				winners.Add(1)
			case errors.Is(err, ErrKycIdentityConflict):
				identityConflicts.Add(1)
			default:
				unexpectedFailures.Add(1)
			}
		}()
	}
	for i := 0; i < workers; i++ {
		<-ready
	}
	close(start)
	waitGroup.Wait()
	require.Equal(t, int32(1), winners.Load())
	require.Equal(t, int32(workers-1), identityConflicts.Load())
	require.Zero(t, unexpectedFailures.Load())

	var count int64
	require.NoError(t, db.Model(&model.Kyc{}).Where("email = ?", email).Count(&count).Error)
	require.Equal(t, int64(1), count)

	stored, found, err := GetKycByEmail(email)
	require.NoError(t, err)
	require.True(t, found)
	require.NotEqual(t, uuid.Nil, stored.Uuid)
}
