package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestVerificationSessionProviderScopedUniquenessAndLatestLookup(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	kycUuid := uuid.New()
	providerSessionId := "session-" + uuid.NewString()
	createVerificationTestKyc(t, db, kycUuid)
	t.Cleanup(func() {
		require.NoError(t, db.Where("kyc_uuid = ?", kycUuid).Delete(&model.VerificationSession{}).Error)
		require.NoError(t, db.Where("uuid = ?", kycUuid).Delete(&model.Kyc{}).Error)
	})

	first := verificationSessionFixture(kycUuid, providerSessionId, model.VerificationEnvironmentSandbox)
	require.NoError(t, CreateVerificationSession(first))
	require.ErrorContains(
		t,
		db.Where("uuid = ?", kycUuid).Delete(&model.Kyc{}).Error,
		"violates foreign key constraint",
	)

	duplicate := verificationSessionFixture(kycUuid, providerSessionId, model.VerificationEnvironmentSandbox)
	require.Error(t, CreateVerificationSession(duplicate))

	production := verificationSessionFixture(kycUuid, providerSessionId, model.VerificationEnvironmentProduction)
	production.CreatedAt = first.CreatedAt.Add(time.Minute)
	require.NoError(t, CreateVerificationSession(production))

	latestSandbox := verificationSessionFixture(
		kycUuid,
		"session-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
	)
	latestSandbox.CreatedAt = first.CreatedAt.Add(2 * time.Minute)
	require.NoError(t, CreateVerificationSession(latestSandbox))

	stored, found, err := GetVerificationSession(
		model.VerificationProviderDidit,
		model.VerificationEnvironmentSandbox,
		providerSessionId,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.Uuid, stored.Uuid)

	latest, found, err := GetLatestVerificationSession(
		kycUuid,
		model.IndividualCustomer,
		model.VerificationProviderDidit,
		model.VerificationEnvironmentSandbox,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, latestSandbox.Uuid, latest.Uuid)
}

func TestUpdateVerificationSessionPersistsEmptyAndNilValues(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	kycUuid := uuid.New()
	createVerificationTestKyc(t, db, kycUuid)
	session := verificationSessionFixture(
		kycUuid,
		"session-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
	)
	decisionAt := time.Now().UTC().Truncate(time.Microsecond)
	session.ProviderApplicationId = "application"
	session.WorkflowId = "workflow"
	session.WorkflowVersion = "1"
	session.ProviderStatus = "Approved"
	session.DecisionAt = &decisionAt
	session.LastReconciledAt = &decisionAt
	require.NoError(t, CreateVerificationSession(session))
	t.Cleanup(func() {
		require.NoError(t, db.Where("uuid = ?", session.Uuid).Delete(&model.VerificationSession{}).Error)
		require.NoError(t, db.Where("uuid = ?", kycUuid).Delete(&model.Kyc{}).Error)
	})

	session.ProviderApplicationId = ""
	session.WorkflowId = ""
	session.WorkflowVersion = ""
	session.ApplicantType = model.BusinessCustomer
	session.DecisionAt = nil
	session.LastReconciledAt = nil
	session.UpdatedAt = decisionAt.Add(time.Minute)
	require.NoError(t, UpdateVerificationSession(session))

	stored, found, err := GetVerificationSession(session.Provider, session.Environment, session.ProviderSessionId)
	require.NoError(t, err)
	require.True(t, found)
	require.Empty(t, stored.ProviderApplicationId)
	require.Equal(t, "workflow", stored.WorkflowId)
	require.Equal(t, "1", stored.WorkflowVersion)
	require.Equal(t, model.IndividualCustomer, stored.ApplicantType)
	require.Equal(t, model.StatusInit, stored.KycStatus)
	require.Equal(t, "Approved", stored.ProviderStatus)
	require.Nil(t, stored.DecisionAt)
	require.Nil(t, stored.LastReconciledAt)
}

func TestCreateVerificationSessionRejectsMissingIdentity(t *testing.T) {
	requireStorageTestDatabase(t)

	session := verificationSessionFixture(
		uuid.Nil,
		"",
		model.VerificationEnvironmentSandbox,
	)
	require.Error(t, CreateVerificationSession(session))
}

func TestCreateVerificationSessionRejectsDiditWithoutWorkflowId(t *testing.T) {
	requireStorageTestDatabase(t)

	session := verificationSessionFixture(
		uuid.New(),
		"session-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
	)
	session.WorkflowId = ""
	require.ErrorContains(t, CreateVerificationSession(session), "require a workflow id")
}

func TestCreateVerificationSessionRejectsUnknownKycUuid(t *testing.T) {
	requireStorageTestDatabase(t)

	session := verificationSessionFixture(
		uuid.New(),
		"session-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
	)
	require.ErrorContains(t, CreateVerificationSession(session), "violates foreign key constraint")
}

func TestCreateVerificationWebhookEventIsIdempotent(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	eventId := "event-" + uuid.NewString()
	t.Cleanup(func() {
		require.NoError(t, db.Where("event_id = ?", eventId).Delete(&model.VerificationWebhookEvent{}).Error)
	})

	first := verificationWebhookEventFixture(eventId, model.VerificationEnvironmentSandbox, "first")
	created, err := CreateVerificationWebhookEvent(first)
	require.NoError(t, err)
	require.True(t, created)

	duplicate := verificationWebhookEventFixture(eventId, model.VerificationEnvironmentSandbox, "first")
	duplicate.PayloadSha256 = strings.ToUpper(duplicate.PayloadSha256)
	created, err = CreateVerificationWebhookEvent(duplicate)
	require.NoError(t, err)
	require.False(t, created)

	production := verificationWebhookEventFixture(eventId, model.VerificationEnvironmentProduction, "production")
	created, err = CreateVerificationWebhookEvent(production)
	require.NoError(t, err)
	require.True(t, created)

	stored, found, err := GetVerificationWebhookEvent(
		model.VerificationProviderDidit,
		model.VerificationEnvironmentSandbox,
		eventId,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.Uuid, stored.Uuid)
	require.Equal(t, first.PayloadSha256, stored.PayloadSha256)
}

func TestCreateVerificationWebhookEventRejectsEventIdWithDifferentPayload(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	eventId := "event-payload-mismatch-" + uuid.NewString()
	t.Cleanup(func() {
		require.NoError(t, db.Where("event_id = ?", eventId).Delete(&model.VerificationWebhookEvent{}).Error)
	})

	first := verificationWebhookEventFixture(eventId, model.VerificationEnvironmentSandbox, "first")
	created, err := CreateVerificationWebhookEvent(first)
	require.NoError(t, err)
	require.True(t, created)

	differentPayload := verificationWebhookEventFixture(eventId, model.VerificationEnvironmentSandbox, "different")
	created, err = CreateVerificationWebhookEvent(differentPayload)
	require.False(t, created)
	require.ErrorIs(t, err, ErrVerificationEventPayloadMismatch)

	stored, found, err := GetVerificationWebhookEvent(
		model.VerificationProviderDidit,
		model.VerificationEnvironmentSandbox,
		eventId,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.PayloadSha256, stored.PayloadSha256)
}

func TestCreateVerificationWebhookEventConcurrentDuplicateHasOneWinner(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)

	eventId := "event-concurrent-" + uuid.NewString()
	t.Cleanup(func() {
		require.NoError(t, db.Where("event_id = ?", eventId).Delete(&model.VerificationWebhookEvent{}).Error)
	})

	const workers = 16
	var createdCount atomic.Int32
	var failures atomic.Int32
	ready := make(chan struct{}, workers)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer waitGroup.Done()
			ready <- struct{}{}
			<-start
			event := verificationWebhookEventFixture(
				eventId,
				model.VerificationEnvironmentSandbox,
				"same-payload",
			)
			created, err := CreateVerificationWebhookEvent(event)
			if err != nil {
				failures.Add(1)
				return
			}
			if created {
				createdCount.Add(1)
			}
		}()
	}
	for i := 0; i < workers; i++ {
		<-ready
	}
	close(start)
	waitGroup.Wait()

	require.Zero(t, failures.Load())
	require.Equal(t, int32(1), createdCount.Load())

	var count int64
	require.NoError(t, db.Model(&model.VerificationWebhookEvent{}).
		Where("provider = ? AND environment = ? AND event_id = ?",
			model.VerificationProviderDidit,
			model.VerificationEnvironmentSandbox,
			eventId,
		).
		Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestCreateVerificationWebhookEventRejectsMissingIdempotencyKey(t *testing.T) {
	requireStorageTestDatabase(t)

	event := verificationWebhookEventFixture("", model.VerificationEnvironmentSandbox, "missing")
	created, err := CreateVerificationWebhookEvent(event)
	require.Error(t, err)
	require.False(t, created)
}

func TestVerificationPersistenceRejectsUnsupportedScopedValues(t *testing.T) {
	requireStorageTestDatabase(t)

	session := verificationSessionFixture(
		uuid.New(),
		"session-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
	)
	session.Provider = "unknown"
	require.ErrorContains(t, CreateVerificationSession(session), "unsupported verification provider")

	event := verificationWebhookEventFixture(
		"event-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
		"unsupported-status",
	)
	event.ProcessingStatus = "unknown"
	created, err := CreateVerificationWebhookEvent(event)
	require.False(t, created)
	require.ErrorContains(t, err, "unsupported verification event status")
}

func verificationSessionFixture(
	kycUuid uuid.UUID,
	providerSessionId, environment string,
) *model.VerificationSession {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &model.VerificationSession{
		KycUuid:           kycUuid,
		Provider:          model.VerificationProviderDidit,
		Environment:       environment,
		ProviderSessionId: providerSessionId,
		WorkflowId:        "workflow-" + uuid.NewString(),
		ApplicantType:     model.IndividualCustomer,
		KycStatus:         model.StatusInit,
		ProviderStatus:    "Not Started",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func verificationWebhookEventFixture(
	eventId, environment, payloadMarker string,
) *model.VerificationWebhookEvent {
	payloadDigest := sha256.Sum256([]byte(payloadMarker))
	return &model.VerificationWebhookEvent{
		Provider:         model.VerificationProviderDidit,
		Environment:      environment,
		EventId:          eventId,
		EventType:        "status.updated",
		PayloadSha256:    hex.EncodeToString(payloadDigest[:]),
		ProcessingStatus: model.VerificationEventReceived,
	}
}

func createVerificationTestKyc(t *testing.T, db *gorm.DB, kycUuid uuid.UUID) {
	t.Helper()
	receiveUpdates := false
	require.NoError(t, db.Create(&model.Kyc{
		Uuid:           kycUuid,
		Email:          fmt.Sprintf("verification-kyc-%s@example.com", kycUuid),
		KycStatus:      model.StatusInit,
		ReceiveUpdates: &receiveUpdates,
	}).Error)
}
