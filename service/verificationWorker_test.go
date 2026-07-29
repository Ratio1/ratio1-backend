package service

import (
	"context"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/stretchr/testify/require"
)

func TestProjectableDiditEntityStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected DiditEntityStatus
		valid    bool
	}{
		{input: "ACTIVE", expected: DiditEntityActive, valid: true},
		{input: " approved ", expected: DiditEntityActive, valid: true},
		{input: "FLAGGED", expected: DiditEntityFlagged, valid: true},
		{input: "in review", expected: DiditEntityFlagged, valid: true},
		{input: "pending", expected: DiditEntityFlagged, valid: true},
		{input: "BLOCKED", expected: DiditEntityBlocked, valid: true},
		{input: "declined", expected: DiditEntityBlocked, valid: true},
		{input: "", valid: false},
		{input: "unknown", valid: false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			status, err := projectableDiditEntityStatus(test.input)
			if test.valid {
				require.NoError(t, err)
				require.Equal(t, test.expected, status)
			} else {
				require.ErrorContains(t, err, "unknown Didit entity status")
				require.Empty(t, status)
			}
		})
	}
}

func TestDiditDecisionTerminalAndNotificationProjection(t *testing.T) {
	terminal := []model.DiditSessionStatus{
		model.DiditStatusApproved,
		model.DiditStatusDeclined,
		model.DiditStatusExpired,
		model.DiditStatusKycExpired,
		model.DiditStatusAbandoned,
	}
	for _, status := range terminal {
		require.True(t, diditDecisionIsTerminal(status), status)
	}
	nonTerminal := []model.DiditSessionStatus{
		model.DiditStatusNotStarted,
		model.DiditStatusInProgress,
		model.DiditStatusAwaitingUser,
		model.DiditStatusInReview,
		model.DiditStatusResubmitted,
		"",
	}
	for _, status := range nonTerminal {
		require.False(t, diditDecisionIsTerminal(status), status)
	}

	require.Equal(t, model.VerificationNotificationApproved, diditNotificationType(model.StatusApproved))
	require.Equal(t, model.VerificationNotificationFinalRejected, diditNotificationType(model.StatusFinalRejected))
	require.Equal(t, model.VerificationNotificationRetry, diditNotificationType(model.StatusRejected))
	require.Empty(t, diditNotificationType(model.StatusOnHold))
	require.Empty(t, diditNotificationType(model.StatusPending))
}

func TestVerificationRetryDelayIsBoundedExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempt  uint
		expected time.Duration
	}{
		{attempt: 0, expected: time.Minute},
		{attempt: 1, expected: time.Minute},
		{attempt: 2, expected: 2 * time.Minute},
		{attempt: 3, expected: 4 * time.Minute},
		{attempt: 8, expected: 128 * time.Minute},
		{attempt: 9, expected: 128 * time.Minute},
		{attempt: 100, expected: 128 * time.Minute},
	}
	for _, test := range tests {
		require.Equal(t, test.expected, verificationRetryDelay(test.attempt))
	}
}

func TestInactiveVerificationWorkerStopsWithoutStartingDatabaseWork(t *testing.T) {
	worker := (&VerificationService{}).StartWorker(context.Background())
	require.NotNil(t, worker)
	require.Eventually(t, func() bool {
		select {
		case <-worker.done:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	worker.Stop()
	worker.Stop()
}
