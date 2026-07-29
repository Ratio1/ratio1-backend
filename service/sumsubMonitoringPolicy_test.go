package service

import (
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/stretchr/testify/require"
)

func TestPrepareSumsubKycForFullProcessingPreservesPreCutoverOwnership(t *testing.T) {
	for _, provider := range []string{"", model.VerificationProviderSumsub} {
		kyc := model.Kyc{VerificationProvider: provider}
		prepared, process := PrepareSumsubKycForFullProcessing(kyc)
		require.True(t, process)
		require.Equal(t, model.VerificationProviderSumsub, prepared.VerificationProvider)
	}

	diditOwned := model.Kyc{VerificationProvider: model.VerificationProviderDidit}
	prepared, process := PrepareSumsubKycForFullProcessing(diditOwned)
	require.False(t, process)
	require.Equal(t, diditOwned, prepared)
}

func TestGrandfatheredSumsubMonitoringNeverReapprovesOrResets(t *testing.T) {
	lastUpdated := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	base := model.Kyc{
		ApplicantId:          "sumsub-applicant",
		VerificationProvider: model.VerificationProviderSumsub,
		KycStatus:            model.StatusOnHold,
		LastUpdated:          lastUpdated,
		IsActive:             false,
	}
	baseEvent := model.SumsubEvent{
		ApplicantID: "sumsub-applicant",
		CreatedAtMs: "2026-07-29 11:00:00.000",
	}

	tests := []struct {
		name  string
		event model.SumsubEvent
	}{
		{
			name: "green review cannot reapprove",
			event: func() model.SumsubEvent {
				event := baseEvent
				event.Type = model.ApplicantReviewed
				event.ReviewResult.ReviewAnswer = "GREEN"
				return event
			}(),
		},
		{
			name: "applicant reset is ignored",
			event: func() model.SumsubEvent {
				event := baseEvent
				event.Type = model.ApplicantReset
				return event
			}(),
		},
		{
			name: "activation is ignored",
			event: func() model.SumsubEvent {
				event := baseEvent
				event.Type = model.ApplicantActivated
				return event
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, changed, err := grandfatheredSumsubMonitoringTransition(test.event, base)
			require.NoError(t, err)
			require.False(t, changed)
			require.Equal(t, base, result)
		})
	}
}

func TestGrandfatheredSumsubMonitoringAllowsOnlySuspensionOrRevocation(t *testing.T) {
	lastUpdated := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	base := model.Kyc{
		ApplicantId:          "sumsub-applicant",
		VerificationProvider: model.VerificationProviderSumsub,
		KycStatus:            model.StatusApproved,
		LastUpdated:          lastUpdated,
		IsActive:             true,
	}
	baseEvent := model.SumsubEvent{
		ApplicantID: "sumsub-applicant",
		CreatedAtMs: "2026-07-29 11:00:00.000",
	}

	suspended := baseEvent
	suspended.Type = model.ApplicantReviewed
	suspended.ReviewResult.ReviewAnswer = "RED"
	suspended.ReviewResult.ReviewRejectType = "RETRY"
	result, changed, err := grandfatheredSumsubMonitoringTransition(suspended, base)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.StatusOnHold, result.KycStatus)

	revoked := baseEvent
	revoked.Type = model.ApplicantActionReviewed
	revoked.ReviewResult.ReviewAnswer = "RED"
	revoked.ReviewResult.ReviewRejectType = "FINAL"
	result, changed, err = grandfatheredSumsubMonitoringTransition(revoked, base)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.StatusFinalRejected, result.KycStatus)

	deactivated := baseEvent
	deactivated.Type = model.ApplicantDeactivated
	result, changed, err = grandfatheredSumsubMonitoringTransition(deactivated, base)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.StatusOnHold, result.KycStatus)
	require.False(t, result.IsActive)

	deleted := baseEvent
	deleted.Type = model.ApplicantDeleted
	result, changed, err = grandfatheredSumsubMonitoringTransition(deleted, base)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.StatusOnHold, result.KycStatus)
	require.True(t, result.HasBeenDeleted)
}

func TestGrandfatheredSumsubMonitoringRejectsStaleOrMismatchedEvents(t *testing.T) {
	base := model.Kyc{
		ApplicantId:          "sumsub-applicant",
		VerificationProvider: model.VerificationProviderSumsub,
		KycStatus:            model.StatusApproved,
		LastUpdated:          time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
	}
	event := model.SumsubEvent{
		ApplicantID: "sumsub-applicant",
		Type:        model.ApplicantReviewed,
		CreatedAtMs: "2026-07-29 11:00:00.000",
	}
	event.ReviewResult.ReviewAnswer = "RED"
	event.ReviewResult.ReviewRejectType = "FINAL"

	result, changed, err := grandfatheredSumsubMonitoringTransition(event, base)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, base, result)

	event.CreatedAtMs = "2026-07-29 13:00:00.000"
	event.ApplicantID = "different-applicant"
	result, changed, err = grandfatheredSumsubMonitoringTransition(event, base)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, base, result)

	diditOwned := base
	diditOwned.VerificationProvider = model.VerificationProviderDidit
	event.ApplicantID = diditOwned.ApplicantId
	result, changed, err = grandfatheredSumsubMonitoringTransition(event, diditOwned)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, diditOwned, result)

	event.ApplicantID = base.ApplicantId
	event.CreatedAtMs = "2026-07-29 12:00:00.000"
	result, changed, err = grandfatheredSumsubMonitoringTransition(event, base)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, base, result)
}
