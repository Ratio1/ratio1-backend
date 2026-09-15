package service

import (
	"encoding/json"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/stretchr/testify/require"
)

func TestClassifyDiditDecline(t *testing.T) {
	policy := DefaultDiditDeclinePolicy()
	tests := []struct {
		name       string
		fixture    string
		decision   model.DiditDecision
		expected   DiditDeclineDisposition
		projection string
		reason     DiditProjectionReason
	}{
		{
			name:       "jurisdiction minimum age failure is final",
			fixture:    "decision_user_declined_minimum_age.json",
			expected:   DiditDeclineFinal,
			projection: model.StatusFinalRejected,
			reason:     DiditReasonDeclineFinal,
		},
		{
			name:       "expired identity document can retry",
			fixture:    "decision_user_declined_expired_document.json",
			expected:   DiditDeclineRetryable,
			projection: model.StatusRejected,
			reason:     DiditReasonDeclineRetryable,
		},
		{
			name:       "blocked business country is final",
			fixture:    "decision_business_declined_blocked_country.json",
			expected:   DiditDeclineFinal,
			projection: model.StatusFinalRejected,
			reason:     DiditReasonDeclineFinal,
		},
		{
			name: "confirmed AML hit is final",
			decision: model.DiditDecision{
				Status: model.DiditStatusDeclined,
				AmlScreenings: []model.DiditAmlScreening{{
					Status:    "Declined",
					TotalHits: diditIntPointer(1),
				}},
			},
			expected:   DiditDeclineFinal,
			projection: model.StatusFinalRejected,
			reason:     DiditReasonDeclineFinal,
		},
		{
			name: "approved decision with final reason code is final",
			decision: model.DiditDecision{
				Status:             model.DiditStatusApproved,
				DecisionReasonCode: "blocked_business",
			},
			expected:   DiditDeclineFinal,
			projection: model.StatusFinalRejected,
			reason:     DiditReasonApprovalPolicyRejected,
		},
		{
			name: "approved decision with minimum age warning is final",
			decision: model.DiditDecision{
				Status: model.DiditStatusApproved,
				IdVerifications: []model.DiditIdVerification{{
					Warnings: []model.DiditWarning{{
						Risk:    "MINIMUM_AGE_NOT_MET",
						LogType: "error",
					}},
				}},
			},
			expected:   DiditDeclineFinal,
			projection: model.StatusFinalRejected,
			reason:     DiditReasonApprovalPolicyRejected,
		},
		{
			name: "approved decision with confirmed AML hit is final",
			decision: model.DiditDecision{
				Status: model.DiditStatusApproved,
				AmlScreenings: []model.DiditAmlScreening{{
					Status:    "Declined",
					TotalHits: diditIntPointer(1),
				}},
			},
			expected:   DiditDeclineFinal,
			projection: model.StatusFinalRejected,
			reason:     DiditReasonApprovalPolicyRejected,
		},
		{
			name: "incomplete business documents can retry",
			decision: model.DiditDecision{
				Status:             model.DiditStatusDeclined,
				DecisionReasonCode: "documents_incomplete",
			},
			expected:   DiditDeclineRetryable,
			projection: model.StatusRejected,
			reason:     DiditReasonDeclineRetryable,
		},
		{
			name: "unknown reason fails closed",
			decision: model.DiditDecision{
				Status:             model.DiditStatusDeclined,
				DecisionReasonCode: "future_provider_reason",
			},
			expected:   DiditDeclineUnknown,
			projection: model.StatusOnHold,
			reason:     DiditReasonDeclineUnclassified,
		},
		{
			name: "unknown error warning overrides retry signal",
			decision: model.DiditDecision{
				Status:             model.DiditStatusDeclined,
				DecisionReasonCode: "documents_incomplete",
				IdVerifications: []model.DiditIdVerification{{
					Warnings: []model.DiditWarning{{
						Risk:    "FUTURE_PROVIDER_RISK",
						LogType: "error",
					}},
				}},
			},
			expected:   DiditDeclineUnknown,
			projection: model.StatusOnHold,
			reason:     DiditReasonDeclineUnclassified,
		},
		{
			name: "non-error warning does not classify decline",
			decision: model.DiditDecision{
				Status: model.DiditStatusDeclined,
				IdVerifications: []model.DiditIdVerification{{
					Warnings: []model.DiditWarning{{
						Risk:    "DOCUMENT_EXPIRED",
						LogType: "warning",
					}},
				}},
			},
			expected:   DiditDeclineUnknown,
			projection: model.StatusOnHold,
			reason:     DiditReasonDeclineUnclassified,
		},
		{
			name: "non-declined decision is not classified",
			decision: model.DiditDecision{
				Status: model.DiditStatusApproved,
			},
			expected: DiditDeclineUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := test.decision
			if test.fixture != "" {
				require.NoError(t, json.Unmarshal(readDiditFixture(t, test.fixture), &decision))
			}

			disposition := ClassifyDiditDecline(decision, policy, DiditApprovalPolicy{})
			require.Equal(t, test.expected, disposition)
			if test.projection == "" {
				return
			}

			projection := ProjectDiditLifecycle(DiditLifecycleProjectionInput{
				SessionStatus:      decision.Status,
				DeclineDisposition: disposition,
			})
			require.Equal(t, test.projection, projection.KycStatus)
			require.Equal(t, test.reason, projection.Reason)
			require.False(t, projection.GrantsAccess())
		})
	}
}

func TestClassifyDiditResidenceCountryPolicy(t *testing.T) {
	var decision model.DiditDecision
	require.NoError(t, json.Unmarshal(readDiditFixture(t, "decision_user_approved.json"), &decision))
	diditSetQuestionnaireAnswer(t, &decision, diditTestResidenceCountryQuestionId, "USA")

	disposition := ClassifyDiditDecline(
		decision,
		DefaultDiditDeclinePolicy(),
		diditTestKycApprovalPolicy(),
	)
	require.Equal(t, DiditDeclineFinal, disposition)

	projection := ProjectDiditLifecycle(DiditLifecycleProjectionInput{
		SessionStatus:      decision.Status,
		Evidence:           DiditEvidenceIncomplete,
		DeclineDisposition: disposition,
	})
	require.Equal(t, model.StatusFinalRejected, projection.KycStatus)
	require.Equal(t, DiditReasonApprovalPolicyRejected, projection.Reason)
	require.False(t, projection.GrantsAccess())
}

func TestDefaultDiditDeclinePolicyKnownKYBReasonCodes(t *testing.T) {
	policy := DefaultDiditDeclinePolicy()
	for _, reasonCode := range []string{
		"blocked_country",
		"blocked_business",
		"registry_company_dissolved",
		"aml_confirmed_sanction",
		"aml_confirmed_pep",
		"analyst_rejected",
	} {
		require.Equal(t, DiditDeclineFinal, ClassifyDiditDecline(model.DiditDecision{
			Status:             model.DiditStatusDeclined,
			DecisionReasonCode: reasonCode,
		}, policy, DiditApprovalPolicy{}), reasonCode)
	}
}

func TestDefaultDiditDeclinePolicyKnownRetryableWarnings(t *testing.T) {
	policy := DefaultDiditDeclinePolicy()
	tests := []struct {
		risk    string
		feature string
	}{
		{risk: "NO_FACE_DETECTED", feature: "liveness"},
		{risk: "LOW_LIVENESS_SCORE", feature: "liveness"},
		{risk: "MISSING_ADDRESS_INFORMATION", feature: "proof of address"},
		{risk: "POA_DOCUMENT_EXPIRED", feature: "proof of address"},
		{risk: "INVALID_DOCUMENT_TYPE", feature: "proof of address"},
		{risk: "UNABLE_TO_VALIDATE_DOCUMENT_AGE", feature: "proof of address"},
		{risk: "POA_MAX_ATTEMPTS_EXCEEDED", feature: "proof of address"},
	}
	for _, test := range tests {
		decision := model.DiditDecision{Status: model.DiditStatusDeclined}
		warning := model.DiditWarning{Risk: test.risk, LogType: "error"}
		switch test.feature {
		case "liveness":
			decision.LivenessChecks = []model.DiditFeatureResult{{Warnings: []model.DiditWarning{warning}}}
		case "proof of address":
			decision.PoaVerifications = []model.DiditPoaVerification{{Warnings: []model.DiditWarning{warning}}}
		default:
			t.Fatalf("unknown test feature %q", test.feature)
		}
		require.Equal(
			t,
			DiditDeclineRetryable,
			ClassifyDiditDecline(decision, policy, DiditApprovalPolicy{}),
			test.risk,
		)
	}
}

func TestDefaultDiditDeclinePolicyMissingDocumentsCanRetry(t *testing.T) {
	policy := DefaultDiditDeclinePolicy()
	for _, reasonCode := range []string{
		"documents_incomplete",
		"key_people_incomplete",
	} {
		require.Equal(t, DiditDeclineRetryable, ClassifyDiditDecline(model.DiditDecision{
			Status:             model.DiditStatusDeclined,
			DecisionReasonCode: reasonCode,
		}, policy, DiditApprovalPolicy{}), reasonCode)
	}
}

func TestRatio1RestrictedResidenceCountries(t *testing.T) {
	restricted := Ratio1RestrictedResidenceCountries()
	require.Len(t, restricted, 57)
	for _, country := range []string{"AFG", "IRN", "PRK", "RUS", "USA", "VEN"} {
		_, found := restricted[country]
		require.True(t, found, country)
	}
	for _, country := range []string{"ITA", "ESP", "GBR"} {
		_, found := restricted[country]
		require.False(t, found, country)
	}
}

func TestDefaultDiditDeclinePolicyKnownFinalKYCWarnings(t *testing.T) {
	policy := DefaultDiditDeclinePolicy()
	for _, risk := range []string{
		"MINIMUM_AGE_NOT_MET",
		"AGE_BELOW_MINIMUM",
		"COUNTRY_NOT_ALLOWED",
	} {
		require.Equal(t, DiditDeclineFinal, ClassifyDiditDecline(model.DiditDecision{
			Status: model.DiditStatusDeclined,
			IdVerifications: []model.DiditIdVerification{{
				Warnings: []model.DiditWarning{{
					Risk:    risk,
					LogType: "error",
				}},
			}},
		}, policy, DiditApprovalPolicy{}), risk)
	}
}
