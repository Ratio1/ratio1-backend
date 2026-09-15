package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const diditWebhookValidationSecret = "webhook_test_secret_not_for_runtime"

func TestDiditWebhookRejectsUnsupportedEventBeforePersistence(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	timestamp := now.Unix()
	body := fmt.Sprintf(`{
		"event_id":"00000000-0000-4000-8000-000000000401",
		"webhook_type":"account.status.updated",
		"timestamp":%d,
		"created_at":%d,
		"application_id":"00000000-0000-4000-8000-000000000501",
		"environment":"sandbox",
		"session_id":"00000000-0000-4000-8000-000000000101",
		"session_kind":"user",
		"vendor_data":"00000000-0000-4000-8000-000000000301",
		"status":"Approved"
	}`, timestamp, timestamp)
	simplePayload := fmt.Sprintf(
		"%d:%s:%s:%s",
		timestamp,
		"00000000-0000-4000-8000-000000000101",
		"Approved",
		"account.status.updated",
	)
	service := diditWebhookValidationService()

	_, err := service.ReceiveDiditWebhook(
		[]byte(body),
		DiditWebhookHeaders{
			Timestamp: fmt.Sprintf("%d", timestamp),
			Simple:    signDiditWebhookValidationPayload(simplePayload),
		},
		now,
	)
	require.ErrorContains(t, err, "unsupported Didit verification event")
}

func TestDiditSimpleSignatureCannotBypassRequiredSessionEnvelope(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	timestamp := now.Unix()
	body := fmt.Sprintf(`{
		"event_id":"00000000-0000-4000-8000-000000000401",
		"webhook_type":"status.updated",
		"timestamp":%d,
		"created_at":%d,
		"application_id":"00000000-0000-4000-8000-000000000501",
		"environment":"sandbox",
		"session_kind":"user",
		"vendor_data":"00000000-0000-4000-8000-000000000301",
		"status":"Approved"
	}`, timestamp, timestamp)
	simplePayload := fmt.Sprintf("%d::Approved:status.updated", timestamp)
	service := diditWebhookValidationService()

	_, err := service.ReceiveDiditWebhook(
		[]byte(body),
		DiditWebhookHeaders{
			Timestamp: fmt.Sprintf("%d", timestamp),
			Simple:    signDiditWebhookValidationPayload(simplePayload),
		},
		now,
	)
	require.ErrorIs(t, err, ErrDiditWebhookEnvelope)
}

func TestDiditWebhookRejectsEventFamilySessionKindMismatch(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	timestamp := now.Unix()
	body := fmt.Sprintf(`{
		"event_id":"00000000-0000-4000-8000-000000000401",
		"webhook_type":"user.status.updated",
		"timestamp":%d,
		"created_at":%d,
		"application_id":"00000000-0000-4000-8000-000000000501",
		"environment":"sandbox",
		"session_kind":"business",
		"vendor_data":"00000000-0000-4000-8000-000000000301",
		"status":"Approved"
	}`, timestamp, timestamp)
	simplePayload := fmt.Sprintf("%d::Approved:user.status.updated", timestamp)
	service := diditWebhookValidationService()

	_, err := service.ReceiveDiditWebhook(
		[]byte(body),
		DiditWebhookHeaders{
			Timestamp: fmt.Sprintf("%d", timestamp),
			Simple:    signDiditWebhookValidationPayload(simplePayload),
		},
		now,
	)
	require.ErrorIs(t, err, ErrDiditWebhookEnvelope)
}

func TestDiditWebhookEventFamilyClassification(t *testing.T) {
	require.True(t, isDiditSessionEvent("status.updated"))
	require.True(t, isDiditSessionEvent("data.updated"))
	require.False(t, isDiditSessionEvent("user.status.updated"))
	require.False(t, isDiditSessionEvent("user.data.updated"))
	require.False(t, isDiditSessionEvent("business.status.updated"))
	require.False(t, isDiditSessionEvent("business.data.updated"))
	require.False(t, isDiditSessionEvent("unknown"))

	require.Equal(t, model.DiditSessionKindUser, diditSessionKindForEvent("user.status.updated"))
	require.Equal(t, model.DiditSessionKindBusiness, diditSessionKindForEvent("business.data.updated"))
	require.Empty(t, diditSessionKindForEvent("status.updated"))
}

func TestInternalDiditEnvironmentNormalizesKnownProviderValues(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		{input: " sandbox ", expected: model.VerificationEnvironmentSandbox, valid: true},
		{input: "live", expected: model.VerificationEnvironmentProduction, valid: true},
		{input: "PRODUCTION", expected: model.VerificationEnvironmentProduction, valid: true},
		{input: "test", valid: false},
		{input: "", valid: false},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			environment, err := internalDiditEnvironment(test.input)
			if test.valid {
				require.NoError(t, err)
				require.Equal(t, test.expected, environment)
			} else {
				require.Error(t, err)
				require.Empty(t, environment)
			}
		})
	}
}

func diditWebhookValidationService() *VerificationService {
	return &VerificationService{
		cfg: config.GeneralConfig{Didit: config.DiditConfig{
			ApplicationId: "00000000-0000-4000-8000-000000000501",
			Environment:   model.VerificationEnvironmentSandbox,
			WebhookSecret: diditWebhookValidationSecret,
		}},
		didit: &diditWebhookValidationClient{},
	}
}

type diditWebhookValidationClient struct{}

func (*diditWebhookValidationClient) CreateSession(
	context.Context,
	model.DiditCreateSessionRequest,
) (*model.DiditCreateSessionResponse, error) {
	panic("unexpected CreateSession call")
}

func (*diditWebhookValidationClient) RetrieveDecision(
	context.Context,
	uuid.UUID,
	model.DiditDecisionExpectation,
) (*model.DiditDecision, error) {
	panic("unexpected RetrieveDecision call")
}

func (*diditWebhookValidationClient) RetrieveEntity(
	context.Context,
	model.DiditSessionKind,
	string,
) (*model.DiditEntity, error) {
	panic("unexpected RetrieveEntity call")
}

func signDiditWebhookValidationPayload(payload string) string {
	mac := hmac.New(sha256.New, []byte(diditWebhookValidationSecret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
