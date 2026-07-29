package service

import (
	"encoding/json"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEvaluateDiditApprovalEvidence(t *testing.T) {
	tests := []struct {
		name            string
		fixture         string
		policy          DiditApprovalPolicy
		workflowVersion int
		mutate          func(*model.DiditDecision)
		expected        DiditEvidenceVerdict
	}{
		{
			name:            "complete KYC evidence",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			expected:        DiditEvidenceEligible,
		},
		{
			name:            "incomplete KYC evidence",
			fixture:         "decision_user_approved_incomplete.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			expected:        DiditEvidenceIncomplete,
		},
		{
			name:            "complete KYB evidence",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			expected:        DiditEvidenceEligible,
		},
		{
			name:            "provider approval with AML hit fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.AmlScreenings[0].TotalHits = diditIntPointer(1)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "provider approval without AML hit count fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.AmlScreenings[0].TotalHits = nil
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "provider approval with error warning fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.IdVerifications[0].Warnings = []model.DiditWarning{{
					Risk:    "FUTURE_PROVIDER_RISK",
					LogType: "error",
				}}
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "provider approval without required questionnaire fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.QuestionnaireResponses = nil
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "empty questionnaire sections fail closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.QuestionnaireResponses[0].Sections = nil
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "missing required questionnaire answer fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.QuestionnaireResponses[0].Sections[0].Items[0].Answer = nil
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "restricted residence country fails approval evidence",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				diditSetQuestionnaireAnswer(t, decision, diditTestResidenceCountryQuestionId, "USA")
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "missing mapped identity fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.IdVerifications[0].DateOfBirth = ""
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "legal mononym remains eligible",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.IdVerifications[0].FirstName = ""
				decision.IdVerifications[0].LastName = ""
				decision.IdVerifications[0].FullName = "Sukarno"
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:            "missing mapped proof of address fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.PoaVerifications[0].PoaAddress = ""
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "empty KYB company evidence fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.RegistryChecks[0].Company = json.RawMessage(`{}`)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "KYB company without registration number fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.RegistryChecks[0].Company = json.RawMessage(
					`{"company_name":"Example SRL","country_code":"IT","registration_number":""}`,
				)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:    "optional KYB registration number may be absent",
			fixture: "decision_business_approved.json",
			policy: func() DiditApprovalPolicy {
				policy := diditTestKybApprovalPolicy()
				policy.RequireCompanyRegistrationNumber = false
				return policy
			}(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.RegistryChecks[0].Company = json.RawMessage(
					`{"company_name":"Example SRL","country_code":"IT","registration_number":null}`,
				)
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:            "empty KYB document evidence fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.DocumentVerifications[0].Items = json.RawMessage(`[]`)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "KYB item status defers to approved required group counters",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.DocumentVerifications[0].Items = json.RawMessage(
					`[{"document_type":"LEGAL_PRESENCE","status":"Declined"}]`,
				)
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:            "declined optional KYB document does not block approval",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.DocumentVerifications[0].Items = json.RawMessage(
					`[{"document_type":"LEGAL_PRESENCE","status":"Approved"},{"document_type":"OTHER","status":"Declined"}]`,
				)
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:            "incomplete required KYB document group fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.DocumentVerifications[0].Groups = json.RawMessage(
					`{"LEGAL_PRESENCE":{"total":1,"approved":0,"declined":1},"OWNERSHIP_STRUCTURE":{"total":1,"approved":1}}`,
				)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "partially approved required KYB document group fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.DocumentVerifications[0].Groups = json.RawMessage(
					`{"LEGAL_PRESENCE":{"total":2,"approved":1,"pending":0,"missing":0},"OWNERSHIP_STRUCTURE":{"approved":1,"pending":0,"missing":0}}`,
				)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "empty KYB key people evidence fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.KeyPeopleChecks[0].Registry = json.RawMessage(`[]`)
				decision.KeyPeopleChecks[0].Submitted = json.RawMessage(`[]`)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "blank KYB key person fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.KeyPeopleChecks[0].Registry = json.RawMessage(`{"officers":[{}]}`)
				decision.KeyPeopleChecks[0].Submitted = json.RawMessage(`{"parties":[]}`)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "object-valued KYB roles are accepted",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.KeyPeopleChecks[0].Registry = json.RawMessage(`{"officers":[]}`)
				decision.KeyPeopleChecks[0].Submitted = json.RawMessage(
					`{"parties":[{"name":"Ada Example","roles":[{"role":"ubo","ownership_percent":100}],"requires_verification":true,"kyc_session_status":"Approved"}]}`,
				)
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:            "declined required KYB key person fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.KeyPeopleChecks[0].Submitted = json.RawMessage(
					`{"parties":[{"name":"Ada Example","role":"ubo","requires_verification":true,"kyc_session_status":"Declined"}]}`,
				)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "skipped optional KYB key person does not block approval",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.KeyPeopleChecks[0].Registry = json.RawMessage(
					`{"officers":[{"name":"Optional Secretary","role":"secretary","requires_verification":false,"is_skipped":true,"kyc_session_status":"Declined"}]}`,
				)
				decision.KeyPeopleChecks[0].Submitted = json.RawMessage(`{"parties":[]}`)
				decision.KeyPeopleChecks[0].UboKycSummary = json.RawMessage(
					`{"total":1,"approved":0,"flagged":0,"pending":1}`,
				)
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:            "pending UBO verification fails closed",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.KeyPeopleChecks[0].Submitted = json.RawMessage(
					`{"parties":[{"name":"Ada Example","role":"ubo","requires_verification":true,"kyc_session_status":"In Progress"}]}`,
				)
				decision.KeyPeopleChecks[0].UboKycSummary = json.RawMessage(
					`{"total":1,"approved":0,"flagged":0,"pending":1}`,
				)
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "KYB without linked UBO sessions remains eligible",
			fixture:         "decision_business_approved.json",
			policy:          diditTestKybApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.KeyPeopleChecks[0].UboKycSummary = json.RawMessage(`null`)
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:    "declined newly required phone feature fails closed",
			fixture: "decision_user_approved.json",
			policy: func() DiditApprovalPolicy {
				policy := diditTestKycApprovalPolicy()
				policy.RequiredFeatures = append(policy.RequiredFeatures, "PHONE")
				return policy
			}(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.Features = append(decision.Features, "PHONE")
				decision.PhoneVerifications = []model.DiditFeatureResult{{Status: "Declined"}}
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:    "approved newly required phone feature is eligible",
			fixture: "decision_user_approved.json",
			policy: func() DiditApprovalPolicy {
				policy := diditTestKycApprovalPolicy()
				policy.RequiredFeatures = append(policy.RequiredFeatures, "PHONE")
				return policy
			}(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.Features = append(decision.Features, "PHONE")
				decision.PhoneVerifications = []model.DiditFeatureResult{{Status: "Approved"}}
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:    "approved standard email verification feature is eligible",
			fixture: "decision_user_approved.json",
			policy: func() DiditApprovalPolicy {
				policy := diditTestKycApprovalPolicy()
				policy.RequiredFeatures = append(policy.RequiredFeatures, "EMAIL_VERIFICATION")
				return policy
			}(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.Features = append(decision.Features, "EMAIL_VERIFICATION")
				decision.EmailVerifications = []model.DiditFeatureResult{{Status: "Approved"}}
			},
			expected: DiditEvidenceEligible,
		},
		{
			name:    "unsupported policy feature fails closed",
			fixture: "decision_user_approved.json",
			policy: func() DiditApprovalPolicy {
				policy := diditTestKycApprovalPolicy()
				policy.RequiredFeatures = append(policy.RequiredFeatures, "FUTURE_FEATURE")
				return policy
			}(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.Features = append(decision.Features, "FUTURE_FEATURE")
			},
			expected: DiditEvidenceUnknown,
		},
		{
			name:            "unknown enabled feature fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.Features = append(decision.Features, "FUTURE_FEATURE")
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "missing expected feature fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.Features = decision.Features[:len(decision.Features)-1]
			},
			expected: DiditEvidenceIncomplete,
		},
		{
			name:            "workflow version drift fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 2,
			expected:        DiditEvidenceUnknown,
		},
		{
			name:            "unknown session kind fails closed",
			fixture:         "decision_user_approved.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			mutate: func(decision *model.DiditDecision) {
				decision.SessionKind = ""
			},
			expected: DiditEvidenceUnknown,
		},
		{
			name:            "non-approved session has no approval evidence",
			fixture:         "decision_user_declined_minimum_age.json",
			policy:          diditTestKycApprovalPolicy(),
			workflowVersion: 1,
			expected:        DiditEvidenceUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var decision model.DiditDecision
			require.NoError(t, json.Unmarshal(readDiditFixture(t, test.fixture), &decision))
			if test.mutate != nil {
				test.mutate(&decision)
			}
			require.Equal(
				t,
				test.expected,
				EvaluateDiditApprovalEvidence(decision, test.workflowVersion, test.policy),
			)
		})
	}
}

var (
	diditTestKycQuestionnaireId         = uuid.MustParse("b271880d-15f5-4621-a851-a193b4aebddd")
	diditTestKybQuestionnaireId         = uuid.MustParse("e6c959b9-b5e2-4ba5-9217-92ebe8613f5a")
	diditTestResidenceCountryQuestionId = uuid.MustParse("96646c0d-9650-4844-bd5e-85971060131e")
)

func diditTestKycApprovalPolicy() DiditApprovalPolicy {
	return DiditApprovalPolicy{
		WorkflowId:           diditTestWorkflowId,
		WorkflowVersion:      1,
		SessionKind:          model.DiditSessionKindUser,
		RequiredFeatures:     []string{"ID_VERIFICATION", "LIVENESS", "FACE_MATCH", "POA", "QUESTIONNAIRE", "AML"},
		QuestionnaireId:      diditTestKycQuestionnaireId,
		QuestionnaireVersion: 2,
		RequiredQuestionnaireItems: []uuid.UUID{
			uuid.MustParse("5ab7c537-32b2-4792-b522-b9065da0df3a"),
			uuid.MustParse("6828a11b-1ebf-4999-a942-bfd77dfa948c"),
			uuid.MustParse("18777afd-6fb6-4a8e-817b-0d7be1229dc5"),
			uuid.MustParse("fb48c0f2-46e8-4bcf-88f3-298c37178acb"),
			uuid.MustParse("b8d3600e-f281-4e2a-a964-8ccde079f40a"),
			uuid.MustParse("ac420e57-2192-4457-9f72-8ebcf1615cba"),
			uuid.MustParse("9a0cbc08-9e41-4347-8ef3-5e0c0d4a5459"),
			diditTestResidenceCountryQuestionId,
		},
		ResidenceCountryQuestionId:   diditTestResidenceCountryQuestionId,
		RestrictedResidenceCountries: Ratio1RestrictedResidenceCountries(),
	}
}

func diditTestKybApprovalPolicy() DiditApprovalPolicy {
	return DiditApprovalPolicy{
		WorkflowId:           diditTestBusinessWorkflowId,
		WorkflowVersion:      1,
		SessionKind:          model.DiditSessionKindBusiness,
		RequiredFeatures:     []string{"KYB_REGISTRY", "AML", "KYB_DOCUMENTS", "KYB_KEY_PEOPLE", "QUESTIONNAIRE"},
		QuestionnaireId:      diditTestKybQuestionnaireId,
		QuestionnaireVersion: 1,
		RequiredQuestionnaireItems: []uuid.UUID{
			uuid.MustParse("1c2fd10b-c439-4f53-ba4a-d4581563b52d"),
			uuid.MustParse("4ec8a1c8-1b79-4aeb-a91e-2ed2a24552a0"),
			uuid.MustParse("ecac1d83-3413-4c5d-871a-a492f1a1be5b"),
			uuid.MustParse("a714b441-bfd2-4d35-9248-716e668ce2b9"),
			uuid.MustParse("f15f7874-69a4-45de-9cfe-c7fa854b85f6"),
			uuid.MustParse("bfde3e4c-6148-45cd-a04c-c151dd789930"),
			uuid.MustParse("82cfd962-7414-4f8c-8e8d-9cf1d46589de"),
			uuid.MustParse("77250407-9e4c-4dfe-80a8-3f091890f217"),
			uuid.MustParse("d7517e61-084e-4e8d-b96c-9f137c5b7c81"),
		},
		RequireCompanyRegistrationNumber: true,
	}
}

func diditSetQuestionnaireAnswer(
	t *testing.T,
	decision *model.DiditDecision,
	questionId uuid.UUID,
	value string,
) {
	t.Helper()
	for responseIndex := range decision.QuestionnaireResponses {
		for sectionIndex := range decision.QuestionnaireResponses[responseIndex].Sections {
			items := decision.QuestionnaireResponses[responseIndex].Sections[sectionIndex].Items
			for itemIndex := range items {
				if items[itemIndex].Uuid == questionId {
					items[itemIndex].Answer = &model.DiditQuestionnaireResponseAnswer{Value: &value}
					return
				}
			}
		}
	}
	t.Fatalf("questionnaire item %s not found", questionId)
}

func diditIntPointer(value int) *int {
	return &value
}

func TestProjectDiditLifecycle(t *testing.T) {
	tests := []struct {
		name           string
		input          DiditLifecycleProjectionInput
		expectedStatus string
		expectedReason DiditProjectionReason
		grantsAccess   bool
	}{
		{
			name:           "zero value fails closed",
			input:          DiditLifecycleProjectionInput{},
			expectedStatus: model.StatusOnHold,
			expectedReason: DiditReasonUnknownProviderStatus,
		},
		{
			name: "not started",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusNotStarted,
			},
			expectedStatus: model.StatusInit,
			expectedReason: DiditReasonSessionNotStarted,
		},
		{
			name: "in progress",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusInProgress,
			},
			expectedStatus: model.StatusPending,
			expectedReason: DiditReasonSessionPending,
		},
		{
			name: "awaiting user",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusAwaitingUser,
			},
			expectedStatus: model.StatusPending,
			expectedReason: DiditReasonSessionPending,
		},
		{
			name: "in review",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusInReview,
			},
			expectedStatus: model.StatusOnHold,
			expectedReason: DiditReasonSessionInReview,
		},
		{
			name: "approved without evidence",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusApproved,
				Evidence:      DiditEvidenceUnknown,
			},
			expectedStatus: model.StatusOnHold,
			expectedReason: DiditReasonApprovalEvidenceMissing,
		},
		{
			name: "approved with incomplete evidence",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusApproved,
				Evidence:      DiditEvidenceIncomplete,
			},
			expectedStatus: model.StatusOnHold,
			expectedReason: DiditReasonApprovalEvidenceMissing,
		},
		{
			name: "approved with eligible evidence",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusApproved,
				Evidence:      DiditEvidenceEligible,
			},
			expectedStatus: model.StatusApproved,
			expectedReason: DiditReasonApprovalEvidenceReady,
			grantsAccess:   true,
		},
		{
			name: "approved but backend policy rejects",
			input: DiditLifecycleProjectionInput{
				SessionStatus:      model.DiditStatusApproved,
				Evidence:           DiditEvidenceIncomplete,
				DeclineDisposition: DiditDeclineFinal,
			},
			expectedStatus: model.StatusFinalRejected,
			expectedReason: DiditReasonApprovalPolicyRejected,
		},
		{
			name: "declined unknown",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusDeclined,
			},
			expectedStatus: model.StatusOnHold,
			expectedReason: DiditReasonDeclineUnclassified,
		},
		{
			name: "declined retryable",
			input: DiditLifecycleProjectionInput{
				SessionStatus:      model.DiditStatusDeclined,
				DeclineDisposition: DiditDeclineRetryable,
			},
			expectedStatus: model.StatusRejected,
			expectedReason: DiditReasonDeclineRetryable,
		},
		{
			name: "declined final",
			input: DiditLifecycleProjectionInput{
				SessionStatus:      model.DiditStatusDeclined,
				DeclineDisposition: DiditDeclineFinal,
			},
			expectedStatus: model.StatusFinalRejected,
			expectedReason: DiditReasonDeclineFinal,
		},
		{
			name: "resubmitted",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusResubmitted,
			},
			expectedStatus: model.StatusRejected,
			expectedReason: DiditReasonSessionNeedsRetry,
		},
		{
			name: "expired",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusExpired,
			},
			expectedStatus: model.StatusRejected,
			expectedReason: DiditReasonSessionNeedsRetry,
		},
		{
			name: "KYC expired revokes approval",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusKycExpired,
				Evidence:      DiditEvidenceEligible,
			},
			expectedStatus: model.StatusRejected,
			expectedReason: DiditReasonSessionNeedsRetry,
		},
		{
			name: "abandoned",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusAbandoned,
			},
			expectedStatus: model.StatusRejected,
			expectedReason: DiditReasonSessionNeedsRetry,
		},
		{
			name: "flagged overrides eligible approval",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusApproved,
				EntityStatus:  DiditEntityFlagged,
				Evidence:      DiditEvidenceEligible,
			},
			expectedStatus: model.StatusOnHold,
			expectedReason: DiditReasonEntityFlagged,
		},
		{
			name: "blocked overrides every session outcome",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusInProgress,
				EntityStatus:  DiditEntityBlocked,
			},
			expectedStatus: model.StatusFinalRejected,
			expectedReason: DiditReasonEntityBlocked,
		},
		{
			name: "unknown entity status fails closed",
			input: DiditLifecycleProjectionInput{
				SessionStatus: model.DiditStatusApproved,
				EntityStatus:  "UNKNOWN",
				Evidence:      DiditEvidenceEligible,
			},
			expectedStatus: model.StatusOnHold,
			expectedReason: DiditReasonUnknownEntityStatus,
		},
		{
			name: "unknown session status fails closed",
			input: DiditLifecycleProjectionInput{
				SessionStatus: "Unexpected",
			},
			expectedStatus: model.StatusOnHold,
			expectedReason: DiditReasonUnknownProviderStatus,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projection := ProjectDiditLifecycle(test.input)
			require.Equal(t, test.expectedStatus, projection.KycStatus)
			require.Equal(t, test.expectedReason, projection.Reason)
			require.Equal(t, test.grantsAccess, projection.GrantsAccess())
		})
	}
}
