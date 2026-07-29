package service

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var serviceTestDatabaseOnce sync.Once

func TestRetryableTerminalDiditSessionCreatesOneReplacementConcurrently(t *testing.T) {
	requireServiceTestDatabase(t)

	db, err := storage.GetDB()
	require.NoError(t, err)
	kycUuid := uuid.New()
	workflowId := uuid.New()
	oldSessionId := uuid.New()
	newSessionId := uuid.New()
	email := "didit-retry-" + kycUuid.String() + "@example.test"
	reconciledAt := time.Now().UTC().Add(-time.Minute)
	receiveUpdates := false
	require.NoError(t, db.Create(&model.Kyc{
		Uuid:                 kycUuid,
		Email:                email,
		ApplicantType:        model.IndividualCustomer,
		KycStatus:            model.StatusRejected,
		VerificationProvider: model.VerificationProviderDidit,
		ReceiveUpdates:       &receiveUpdates,
		IsActive:             true,
	}).Error)
	oldSession := &model.VerificationSession{
		Uuid:              uuid.New(),
		KycUuid:           kycUuid,
		Provider:          model.VerificationProviderDidit,
		Environment:       model.VerificationEnvironmentSandbox,
		ProviderSessionId: oldSessionId.String(),
		WorkflowId:        workflowId.String(),
		WorkflowVersion:   "1",
		ApplicantType:     model.IndividualCustomer,
		KycStatus:         model.StatusRejected,
		ProviderStatus:    string(model.DiditStatusDeclined),
		LastReconciledAt:  &reconciledAt,
		CreatedAt:         reconciledAt,
		UpdatedAt:         reconciledAt,
	}
	require.NoError(t, storage.CreateVerificationSession(oldSession))
	t.Cleanup(func() {
		require.NoError(t, db.Where("vendor_data = ?", kycUuid.String()).
			Delete(&model.VerificationWebhookEvent{}).Error)
		require.NoError(t, db.Where("kyc_uuid = ?", kycUuid).
			Delete(&model.VerificationSession{}).Error)
		require.NoError(t, db.Where("uuid = ?", kycUuid).Delete(&model.Kyc{}).Error)
	})

	client := &retryableDiditSessionClient{
		oldSessionId: oldSessionId,
		newSessionId: newSessionId,
		workflowId:   workflowId,
		kycUuid:      kycUuid,
	}
	verificationService := &VerificationService{
		cfg: config.GeneralConfig{
			Verification: config.VerificationConfig{
				Provider: model.VerificationProviderDidit,
			},
			Didit: config.DiditConfig{
				Environment: model.VerificationEnvironmentSandbox,
				CallbackUrl: "https://app.ratio1.ai/profile",
			},
		},
		didit: client,
		diditPolicies: DiditPolicySet{Kyc: DiditVerificationPolicy{
			ApprovalPolicy: DiditApprovalPolicy{
				WorkflowId:      workflowId,
				WorkflowVersion: 1,
				SessionKind:     model.DiditSessionKindUser,
			},
		}},
	}
	kyc, found, err := storage.GetKycByUuid(kycUuid)
	require.NoError(t, err)
	require.True(t, found)

	start := make(chan struct{})
	results := make(chan *VerificationSessionResponse, 2)
	errorsChannel := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for index := 0; index < 2; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			result, createErr := verificationService.createOrResumeDiditSession(
				context.Background(),
				kyc,
				model.IndividualCustomer,
			)
			results <- result
			errorsChannel <- createErr
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)
	close(errorsChannel)

	for createErr := range errorsChannel {
		require.NoError(t, createErr)
	}
	for result := range results {
		require.NotNil(t, result)
		require.Equal(t, newSessionId.String(), result.SessionId)
		require.Equal(t, "https://verify.didit.me/session/retry-token", result.Url)
	}
	require.Equal(t, int32(1), client.createCalls.Load())

	var sessionCount int64
	require.NoError(t, db.Model(&model.VerificationSession{}).
		Where("kyc_uuid = ?", kycUuid).
		Count(&sessionCount).Error)
	require.Equal(t, int64(2), sessionCount)
	persistedKyc, found, err := storage.GetKycByUuid(kycUuid)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, model.StatusInit, persistedKyc.KycStatus)
}

func TestStoredSumsubMonitoringEventIsRestartRecoverable(t *testing.T) {
	requireServiceTestDatabase(t)

	db, err := storage.GetDB()
	require.NoError(t, err)
	kycUuid := uuid.New()
	eventId := "sumsub-recovery-" + uuid.NewString()
	receiveUpdates := false
	occurredAt := time.Now().UTC().Truncate(time.Millisecond)
	require.NoError(t, db.Create(&model.Kyc{
		Uuid:                 kycUuid,
		Email:                "sumsub-recovery-" + kycUuid.String() + "@example.test",
		ApplicantId:          "sumsub-applicant-" + kycUuid.String(),
		ApplicantType:        model.IndividualCustomer,
		KycStatus:            model.StatusApproved,
		VerificationProvider: model.VerificationProviderSumsub,
		ReceiveUpdates:       &receiveUpdates,
		IsActive:             true,
		LastUpdated:          occurredAt.Add(-time.Minute),
	}).Error)
	event := &model.VerificationWebhookEvent{
		Provider:          model.VerificationProviderSumsub,
		Environment:       model.VerificationEnvironmentSandbox,
		EventId:           eventId,
		EventType:         model.ApplicantDeactivated,
		ProviderSessionId: "sumsub-applicant-" + kycUuid.String(),
		VendorData:        kycUuid.String(),
		OccurredAt:        &occurredAt,
		ReceivedAt:        occurredAt,
		PayloadSha256:     strings.Repeat("a", 64),
		ProcessingStatus:  model.VerificationEventReceived,
	}
	created, err := storage.CreateVerificationWebhookEvent(event)
	require.NoError(t, err)
	require.True(t, created)
	t.Cleanup(func() {
		require.NoError(t, db.Where("uuid = ?", event.Uuid).
			Delete(&model.VerificationWebhookEvent{}).Error)
		require.NoError(t, db.Where("uuid = ?", kycUuid).Delete(&model.Kyc{}).Error)
	})

	claimed, err := storage.ClaimVerificationWebhookEvents(
		occurredAt.Add(time.Second),
		time.Minute,
		10,
		3,
	)
	require.NoError(t, err)
	var recovered *model.VerificationWebhookEvent
	for index := range claimed {
		if claimed[index].Uuid == event.Uuid {
			recovered = &claimed[index]
			break
		}
	}
	require.NotNil(t, recovered)

	verificationService := &VerificationService{cfg: config.GeneralConfig{
		Verification: config.VerificationConfig{
			Provider:                    model.VerificationProviderDidit,
			LegacySumsubWebhooksEnabled: true,
		},
	}}
	require.NoError(t, verificationService.reconcileGrandfatheredSumsubEvent(recovered))

	storedKyc, found, err := storage.GetKycByUuid(kycUuid)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, model.StatusOnHold, storedKyc.KycStatus)
	require.False(t, storedKyc.IsActive)
	storedEvent, found, err := storage.GetVerificationWebhookEvent(
		model.VerificationProviderSumsub,
		model.VerificationEnvironmentSandbox,
		eventId,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, model.VerificationEventProcessed, storedEvent.ProcessingStatus)
}

type retryableDiditSessionClient struct {
	oldSessionId uuid.UUID
	newSessionId uuid.UUID
	workflowId   uuid.UUID
	kycUuid      uuid.UUID
	createCalls  atomic.Int32
}

func (client *retryableDiditSessionClient) CreateSession(
	_ context.Context,
	_ model.DiditCreateSessionRequest,
) (*model.DiditCreateSessionResponse, error) {
	client.createCalls.Add(1)
	return &model.DiditCreateSessionResponse{
		SessionId:       client.newSessionId,
		SessionKind:     model.DiditSessionKindUser,
		Url:             "https://verify.didit.me/session/retry-token",
		VendorData:      client.kycUuid.String(),
		Status:          model.DiditStatusNotStarted,
		WorkflowId:      client.workflowId,
		WorkflowVersion: 1,
	}, nil
}

func (client *retryableDiditSessionClient) RetrieveDecision(
	_ context.Context,
	sessionId uuid.UUID,
	_ model.DiditDecisionExpectation,
) (*model.DiditDecision, error) {
	if sessionId == client.oldSessionId {
		return &model.DiditDecision{
			SessionId:   sessionId,
			SessionKind: model.DiditSessionKindUser,
			Status:      model.DiditStatusDeclined,
			WorkflowId:  client.workflowId,
			VendorData:  client.kycUuid.String(),
		}, nil
	}
	return &model.DiditDecision{
		SessionId:   sessionId,
		SessionKind: model.DiditSessionKindUser,
		SessionUrl:  "https://verify.didit.me/session/retry-token",
		Status:      model.DiditStatusNotStarted,
		WorkflowId:  client.workflowId,
		VendorData:  client.kycUuid.String(),
	}, nil
}

func (*retryableDiditSessionClient) RetrieveEntity(
	context.Context,
	model.DiditSessionKind,
	string,
) (*model.DiditEntity, error) {
	panic("unexpected RetrieveEntity call")
}

func requireServiceTestDatabase(t *testing.T) {
	t.Helper()
	if os.Getenv("RATIO1_SERVICE_TEST_DATABASE") != "1" {
		t.Skip("set RATIO1_SERVICE_TEST_DATABASE=1 to run PostgreSQL service tests")
	}
	host := serviceTestEnvOrDefault("RATIO1_TEST_DATABASE_HOST", "127.0.0.1")
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		t.Fatal("service integration tests require a loopback database host")
	}
	dbName := serviceTestEnvOrDefault("RATIO1_TEST_DATABASE_NAME", "ratio1_test")
	if !strings.Contains(strings.ToLower(dbName), "test") {
		t.Fatal("service integration test database name must contain test")
	}
	port, err := strconv.Atoi(serviceTestEnvOrDefault("RATIO1_TEST_DATABASE_PORT", "5432"))
	require.NoError(t, err)
	serviceTestDatabaseOnce.Do(func() {
		config.Config.Database = config.DatabaseConfig{
			Host:         host,
			Port:         port,
			User:         serviceTestEnvOrDefault("RATIO1_TEST_DATABASE_USER", "postgres"),
			Password:     serviceTestEnvOrDefault("RATIO1_TEST_DATABASE_PASSWORD", "postgres"),
			DbName:       dbName,
			SslMode:      "disable",
			MaxOpenConns: 10,
			MaxIdleConns: 10,
		}
		storage.Connect()
	})
}

func serviceTestEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
