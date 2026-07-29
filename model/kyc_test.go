package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldPreserveKycOnProviderCutover(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{name: "approved", status: StatusApproved, expected: true},
		{name: "final rejected", status: StatusFinalRejected, expected: true},
		{name: "account created", status: StatusAccountCreated, expected: false},
		{name: "init", status: StatusInit, expected: false},
		{name: "pending", status: StatusPending, expected: false},
		{name: "prechecked", status: StatusPrechecked, expected: false},
		{name: "queued", status: StatusQueued, expected: false},
		{name: "completed", status: StatusCompleted, expected: false},
		{name: "on hold", status: StatusOnHold, expected: false},
		{name: "rejected", status: StatusRejected, expected: false},
		{name: "unknown", status: "unknown", expected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, ShouldPreserveKycOnProviderCutover(test.status))
		})
	}
}
