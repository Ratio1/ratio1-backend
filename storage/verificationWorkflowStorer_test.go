package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestIsSerializationRetryRecognizesWrappedCockroachError(t *testing.T) {
	require.True(t, isSerializationRetry(fmt.Errorf(
		"transaction failed: %w",
		&pq.Error{Code: pq.ErrorCode("40001")},
	)))
	require.False(t, isSerializationRetry(&pq.Error{Code: pq.ErrorCode("23505")}))
	require.False(t, isSerializationRetry(fmt.Errorf("connection unavailable")))
}

func TestRetryVerificationSessionAssignmentRetriesSerializationFailures(t *testing.T) {
	expected := &model.VerificationSession{Uuid: uuid.New()}
	attempts := 0
	stored, err := retryVerificationSessionAssignment(func() (*model.VerificationSession, error) {
		attempts++
		if attempts < 3 {
			return nil, &pq.Error{Code: pq.ErrorCode("40001")}
		}
		return expected, nil
	})

	require.NoError(t, err)
	require.Same(t, expected, stored)
	require.Equal(t, 3, attempts)
}

func TestRetryVerificationSessionAssignmentStopsAfterBoundedAttempts(t *testing.T) {
	attempts := 0
	_, err := retryVerificationSessionAssignment(func() (*model.VerificationSession, error) {
		attempts++
		return nil, &pq.Error{Code: pq.ErrorCode("40001")}
	})

	require.ErrorContains(t, err, "failed after 5 serialization attempts")
	require.Equal(t, verificationSessionAssignmentMaxAttempts, attempts)
}

func TestRetryVerificationSessionAssignmentDoesNotRetryOtherFailures(t *testing.T) {
	attempts := 0
	expected := fmt.Errorf("connection unavailable")
	_, err := retryVerificationSessionAssignment(func() (*model.VerificationSession, error) {
		attempts++
		return nil, expected
	})

	require.ErrorIs(t, err, expected)
	require.Equal(t, 1, attempts)
}

func TestAssignVerificationSessionIsIdempotentAndClaimsProviderOwnership(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)
	kycUuid := uuid.New()
	createVerificationWorkflowTestKyc(t, db, kycUuid, "")
	t.Cleanup(func() {
		cleanupVerificationWorkflowTest(t, db, kycUuid)
	})

	session := verificationSessionFixture(
		kycUuid,
		"session-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
	)
	session.WorkflowVersion = "1"
	first, err := AssignVerificationSession(context.Background(), session)
	require.NoError(t, err)
	second, err := AssignVerificationSession(context.Background(), session)
	require.NoError(t, err)
	require.Equal(t, first.Uuid, second.Uuid)

	var count int64
	require.NoError(t, db.Model(&model.VerificationSession{}).
		Where("kyc_uuid = ?", kycUuid).
		Count(&count).Error)
	require.Equal(t, int64(1), count)

	var kyc model.Kyc
	require.NoError(t, db.Where("uuid = ?", kycUuid).First(&kyc).Error)
	require.Equal(t, model.VerificationProviderDidit, kyc.VerificationProvider)
	require.Equal(t, model.IndividualCustomer, kyc.ApplicantType)
}

func TestAssignVerificationSessionResetsOnlyNewDiditReplacementToInit(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)
	kycUuid := uuid.New()
	createVerificationWorkflowTestKyc(
		t,
		db,
		kycUuid,
		model.VerificationProviderDidit,
	)
	require.NoError(t, db.Model(&model.Kyc{}).
		Where("uuid = ?", kycUuid).
		Updates(map[string]interface{}{
			"applicant_type": model.IndividualCustomer,
			"kyc_status":     model.StatusRejected,
		}).Error)
	t.Cleanup(func() {
		cleanupVerificationWorkflowTest(t, db, kycUuid)
	})

	session := verificationSessionFixture(
		kycUuid,
		"replacement-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
	)
	session.WorkflowVersion = "1"
	_, err = AssignVerificationSession(context.Background(), session)
	require.NoError(t, err)

	var kyc model.Kyc
	require.NoError(t, db.Where("uuid = ?", kycUuid).First(&kyc).Error)
	require.Equal(t, model.StatusInit, kyc.KycStatus)

	require.NoError(t, db.Model(&model.Kyc{}).
		Where("uuid = ?", kycUuid).
		Update("kyc_status", model.StatusRejected).Error)
	_, err = AssignVerificationSession(context.Background(), session)
	require.NoError(t, err)
	require.NoError(t, db.Where("uuid = ?", kycUuid).First(&kyc).Error)
	require.Equal(t, model.StatusRejected, kyc.KycStatus)
}

func TestClaimVerificationWebhookEventsClaimsDiditAndSumsubAndDeadLetters(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)
	eventPrefix := "claim-" + uuid.NewString()
	t.Cleanup(func() {
		require.NoError(t, db.Where("event_id LIKE ?", eventPrefix+"%").
			Delete(&model.VerificationWebhookEvent{}).Error)
	})

	didit := verificationWebhookEventFixture(
		eventPrefix+"-didit",
		model.VerificationEnvironmentSandbox,
		"didit",
	)
	sumsub := verificationWebhookEventFixture(
		eventPrefix+"-sumsub",
		model.VerificationEnvironmentSandbox,
		"sumsub",
	)
	didit.Uuid = uuid.New()
	sumsub.Uuid = uuid.New()
	sumsub.Provider = model.VerificationProviderSumsub
	require.NoError(t, db.Create(didit).Error)
	require.NoError(t, db.Create(sumsub).Error)

	now := time.Now().UTC()
	claimed, err := ClaimVerificationWebhookEvents(now, time.Minute, 10, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 2)
	require.ElementsMatch(t, []string{didit.EventId, sumsub.EventId}, []string{
		claimed[0].EventId,
		claimed[1].EventId,
	})
	require.Equal(t, uint(1), claimed[0].Attempts)
	require.Equal(t, uint(1), claimed[1].Attempts)

	require.NoError(t, MarkVerificationWebhookEventFailed(
		claimed[0].Uuid,
		context.DeadlineExceeded,
		now.Add(time.Minute),
		1,
	))
	var stored model.VerificationWebhookEvent
	require.NoError(t, db.Where("uuid = ?", claimed[0].Uuid).First(&stored).Error)
	require.Equal(t, model.VerificationEventDeadLetter, stored.ProcessingStatus)

	require.NoError(t, RetryDeadLetterVerificationWebhookEvent(stored.Uuid))
	require.NoError(t, db.Where("uuid = ?", stored.Uuid).First(&stored).Error)
	require.Equal(t, model.VerificationEventFailed, stored.ProcessingStatus)
	require.Zero(t, stored.Attempts)
}

func TestApplyVerificationProjectionIsLatestProviderOwnedAndAtomic(t *testing.T) {
	requireStorageTestDatabase(t)

	db, err := GetDB()
	require.NoError(t, err)
	kycUuid := uuid.New()
	createVerificationWorkflowTestKyc(
		t,
		db,
		kycUuid,
		model.VerificationProviderDidit,
	)
	t.Cleanup(func() {
		cleanupVerificationWorkflowTest(t, db, kycUuid)
	})

	session := verificationSessionFixture(
		kycUuid,
		"session-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
	)
	session.WorkflowVersion = "1"
	session.CreatedAt = time.Now().UTC().Add(-time.Minute)
	session.UpdatedAt = session.CreatedAt
	require.NoError(t, CreateVerificationSession(session))
	event := verificationWebhookEventFixture(
		"projection-"+uuid.NewString(),
		model.VerificationEnvironmentSandbox,
		"projection",
	)
	event.ProviderSessionId = session.ProviderSessionId
	event.VendorData = kycUuid.String()
	event.ReceivedAt = time.Now().UTC()
	created, err := CreateVerificationWebhookEvent(event)
	require.NoError(t, err)
	require.True(t, created)

	reconciledAt := time.Now().UTC()
	applied, err := ApplyVerificationProjection(VerificationProjectionUpdate{
		EventUuid:      event.Uuid,
		SessionUuid:    session.Uuid,
		KycStatus:      model.StatusApproved,
		ProviderStatus: string(model.DiditStatusApproved),
		ReconciledAt:   reconciledAt,
	})
	require.ErrorContains(t, err, "requires complete user info")
	require.False(t, applied)

	var storedEvent model.VerificationWebhookEvent
	require.NoError(t, db.Where("uuid = ?", event.Uuid).First(&storedEvent).Error)
	require.Equal(t, model.VerificationEventReceived, storedEvent.ProcessingStatus)

	name := "Ada"
	surname := "Lovelace"
	applied, err = ApplyVerificationProjection(VerificationProjectionUpdate{
		EventUuid:      event.Uuid,
		SessionUuid:    session.Uuid,
		KycStatus:      model.StatusApproved,
		ProviderStatus: string(model.DiditStatusApproved),
		ReconciledAt:   reconciledAt,
		Country:        "GBR",
		UserInfo: &model.UserInfo{
			BlockchainAddress:  "0x0000000000000000000000000000000000000001",
			Email:              "workflow-" + kycUuid.String() + "@example.com",
			Name:               &name,
			Surname:            &surname,
			IdentificationCode: "TAX-001",
			Address:            "1 Test Street",
			State:              "London",
			City:               "London",
			Country:            "GBR",
		},
		NotificationType:          model.VerificationNotificationApproved,
		NotificationTransitionKey: session.Uuid.String() + ":approved",
	})
	require.NoError(t, err)
	require.True(t, applied)

	var kyc model.Kyc
	require.NoError(t, db.Where("uuid = ?", kycUuid).First(&kyc).Error)
	require.Equal(t, model.StatusApproved, kyc.KycStatus)
	require.Equal(t, "GBR", kyc.Country)
	require.NoError(t, db.Where("uuid = ?", event.Uuid).First(&storedEvent).Error)
	require.Equal(t, model.VerificationEventProcessed, storedEvent.ProcessingStatus)

	var notificationCount int64
	require.NoError(t, db.Model(&model.VerificationNotification{}).
		Where("verification_session_uuid = ?", session.Uuid).
		Count(&notificationCount).Error)
	require.Equal(t, int64(1), notificationCount)
}

func createVerificationWorkflowTestKyc(
	t *testing.T,
	db *gorm.DB,
	kycUuid uuid.UUID,
	provider string,
) {
	t.Helper()
	receiveUpdates := false
	require.NoError(t, db.Create(&model.Kyc{
		Uuid:                 kycUuid,
		Email:                "workflow-" + kycUuid.String() + "@example.com",
		KycStatus:            model.StatusAccountCreated,
		VerificationProvider: provider,
		ReceiveUpdates:       &receiveUpdates,
		IsActive:             true,
	}).Error)
}

func cleanupVerificationWorkflowTest(t *testing.T, db *gorm.DB, kycUuid uuid.UUID) {
	t.Helper()
	var sessions []model.VerificationSession
	require.NoError(t, db.Where("kyc_uuid = ?", kycUuid).Find(&sessions).Error)
	for _, session := range sessions {
		require.NoError(t, db.Where("verification_session_uuid = ?", session.Uuid).
			Delete(&model.VerificationNotification{}).Error)
	}
	require.NoError(t, db.Where("vendor_data = ?", kycUuid.String()).
		Delete(&model.VerificationWebhookEvent{}).Error)
	require.NoError(t, db.Where("kyc_uuid = ?", kycUuid).
		Delete(&model.VerificationSession{}).Error)
	require.NoError(t, db.Where("email = ?", "workflow-"+kycUuid.String()+"@example.com").
		Delete(&model.UserInfo{}).Error)
	require.NoError(t, db.Where("uuid = ?", kycUuid).Delete(&model.Kyc{}).Error)
}
