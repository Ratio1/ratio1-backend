package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDiditClientSandboxContract(t *testing.T) {
	if os.Getenv("DIDIT_RUN_SANDBOX_TESTS") != "1" {
		t.Skip("set DIDIT_RUN_SANDBOX_TESTS=1 to exercise the live Didit Sandbox contract")
	}

	apiUrl := os.Getenv("DIDIT_API_URL")
	apiKey := os.Getenv("DIDIT_API_KEY")
	require.NotEmpty(t, apiUrl)
	require.NotEmpty(t, apiKey)

	client, err := NewDiditClient(config.DiditConfig{
		ApiUrl:        apiUrl,
		ApiKey:        apiKey,
		Environment:   model.VerificationEnvironmentSandbox,
		KycWorkflowId: os.Getenv("DIDIT_KYC_WORKFLOW_ID"),
		KybWorkflowId: os.Getenv("DIDIT_KYB_WORKFLOW_ID"),
	}, nil)
	require.NoError(t, err)

	tests := []struct {
		name        string
		workflowEnv string
		kind        model.DiditSessionKind
		vendorId    string
	}{
		{
			name:        "KYC",
			workflowEnv: "DIDIT_KYC_WORKFLOW_ID",
			kind:        model.DiditSessionKindUser,
			vendorId:    "00000000-0000-4000-8000-000000000901",
		},
		{
			name:        "KYB",
			workflowEnv: "DIDIT_KYB_WORKFLOW_ID",
			kind:        model.DiditSessionKindBusiness,
			vendorId:    "00000000-0000-4000-8000-000000000902",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workflowId, err := uuid.Parse(os.Getenv(test.workflowEnv))
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			session, err := client.CreateSession(ctx, model.DiditCreateSessionRequest{
				WorkflowId:          workflowId,
				VendorData:          test.vendorId,
				ExpectedSessionKind: test.kind,
				Metadata: map[string]interface{}{
					"contract_test": true,
				},
			})
			require.NoError(t, err)
			require.Equal(t, workflowId, session.WorkflowId)
			require.Equal(t, test.vendorId, session.VendorData)

			decision, err := client.RetrieveDecision(ctx, session.SessionId, model.DiditDecisionExpectation{
				VendorData:  test.vendorId,
				WorkflowId:  workflowId,
				SessionKind: test.kind,
			})
			require.NoError(t, err)
			require.Equal(t, session.SessionId, decision.SessionId)
			require.Equal(t, test.kind, decision.SessionKind)
			require.Equal(t, model.DiditStatusNotStarted, decision.Status)
		})
	}
}
