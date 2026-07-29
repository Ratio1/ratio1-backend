package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVerificationSessionResponseRequiresProviderSpecificCredential(t *testing.T) {
	tests := []struct {
		name     string
		response VerificationSessionResponse
		valid    bool
	}{
		{
			name: "Didit credential",
			response: VerificationSessionResponse{
				Provider:      model.VerificationProviderDidit,
				ApplicantType: model.IndividualCustomer,
				Status:        model.StatusInit,
				SessionId:     uuid.NewString(),
				Url:           "https://verify.didit.me/session",
			},
			valid: true,
		},
		{
			name: "Sumsub credential",
			response: VerificationSessionResponse{
				Provider:      model.VerificationProviderSumsub,
				ApplicantType: model.BusinessCustomer,
				Status:        model.StatusInit,
				AccessToken:   "token",
			},
			valid: true,
		},
		{
			name: "Didit must include URL",
			response: VerificationSessionResponse{
				Provider:      model.VerificationProviderDidit,
				ApplicantType: model.IndividualCustomer,
				Status:        model.StatusInit,
				SessionId:     uuid.NewString(),
			},
		},
		{
			name: "provider credentials cannot overlap",
			response: VerificationSessionResponse{
				Provider:      model.VerificationProviderDidit,
				ApplicantType: model.IndividualCustomer,
				Status:        model.StatusInit,
				SessionId:     uuid.NewString(),
				Url:           "https://verify.didit.me/session",
				AccessToken:   "token",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.response.Validate()
			if test.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestDiditCreateSessionUsesFixedProfileInitiatorCallback(t *testing.T) {
	policy := DiditVerificationPolicy{
		ApprovalPolicy: DiditApprovalPolicy{
			WorkflowId:  uuid.New(),
			SessionKind: model.DiditSessionKindBusiness,
		},
	}
	kyc := &model.Kyc{
		Uuid:  uuid.New(),
		Email: "company@example.com",
	}

	request := diditCreateSessionRequest(
		"https://app.ratio1.ai/profile",
		kyc,
		model.BusinessCustomer,
		policy,
	)

	require.Equal(t, "https://app.ratio1.ai/profile", request.Callback)
	require.Equal(t, "initiator", request.CallbackMethod)
	require.Equal(t, kyc.Uuid.String(), request.VendorData)
	require.Equal(t, model.DiditSessionKindBusiness, request.ExpectedSessionKind)
	require.Equal(t, model.BusinessCustomer, request.Metadata["ratio1_applicant_type"])
	require.NotContains(t, request.Callback, "url=")
	require.NotContains(t, request.Callback, "token=")
}

func TestDiditHostedUrlAllowlist(t *testing.T) {
	require.True(t, isAllowedDiditHostedURL("https://verify.didit.me/session/token"))
	require.False(t, isAllowedDiditHostedURL("http://verify.didit.me/session/token"))
	require.False(t, isAllowedDiditHostedURL("https://verify.didit.me.attacker.invalid/session/token"))
	require.False(t, isAllowedDiditHostedURL("https://verify.didit.me/other/token"))
	require.False(t, isAllowedDiditHostedURL("https://verify.didit.me/session/"))
	require.False(t, isAllowedDiditHostedURL("https://user@verify.didit.me/session/token"))
}

func TestDiditReplacementRequiresReconciledRetryableSessionAndKyc(t *testing.T) {
	reconciledAt := time.Now().UTC()
	session := &model.VerificationSession{
		KycStatus:        model.StatusRejected,
		ProviderStatus:   string(model.DiditStatusDeclined),
		LastReconciledAt: &reconciledAt,
	}
	decision := &model.DiditDecision{Status: model.DiditStatusDeclined}
	kyc := &model.Kyc{KycStatus: model.StatusRejected}
	require.True(t, diditTerminalSessionWasReconciledAsRetryable(session, decision, kyc))

	finalRejected := *kyc
	finalRejected.KycStatus = model.StatusFinalRejected
	require.False(t, diditTerminalSessionWasReconciledAsRetryable(
		session,
		decision,
		&finalRejected,
	))

	session.LastReconciledAt = nil
	require.False(t, diditTerminalSessionWasReconciledAsRetryable(session, decision, kyc))
}

func TestMapDiditDecisionToUserInfoRequiresStructuredKybBillingAnswers(t *testing.T) {
	var decision model.DiditDecision
	require.NoError(t, json.Unmarshal(readDiditFixture(t, "decision_business_approved.json"), &decision))

	sourceQuestion := uuid.New()
	policy := DiditVerificationPolicy{
		ApprovalPolicy: diditTestKybApprovalPolicy(),
	}
	policy.ApprovalPolicy.RequiredQuestionnaireItems = append(
		policy.ApprovalPolicy.RequiredQuestionnaireItems,
		sourceQuestion,
	)
	appendDiditAnswers(&decision, map[uuid.UUID]string{
		sourceQuestion: "business-income",
	})
	decision.RegistryChecks[0].Company = json.RawMessage(`{
		"company_name":"Example SRL",
		"country_code":"ITA",
		"registration_number":"REG-001",
		"registered_address":"Via Test 1",
		"location_of_registration":"Rome",
		"tax_number":"IT-TAX-001",
		"vat_number":"IT12345678901",
		"vat_validation_status":"valid",
		"user_provided_data":{"region":"RM"}
	}`)

	userInfo, country, viesRegistered, err := MapDiditDecisionToUserInfo(
		decision,
		policy,
		"0x0000000000000000000000000000000000000001",
		"company@example.com",
	)
	require.NoError(t, err)
	require.Equal(t, "IT12345678901", userInfo.IdentificationCode)
	require.Equal(t, "ITA", country)
	require.True(t, viesRegistered)

	missingTax := decision
	missingTax.RegistryChecks[0].Company = json.RawMessage(`{
		"company_name":"Example SRL",
		"country_code":"ITA",
		"registration_number":"REG-001",
		"registered_address":"Via Test 1",
		"location_of_registration":"Rome",
		"vat_number":"IT12345678901",
		"vat_validation_status":"valid",
		"user_provided_data":{"region":"RM"}
	}`)
	_, _, _, err = MapDiditDecisionToUserInfo(
		missingTax,
		policy,
		"0x0000000000000000000000000000000000000001",
		"company@example.com",
	)
	require.ErrorContains(t, err, "invoicing data is incomplete")
}

func TestGrandfatheredSumsubMonitoringIsRevokeOnly(t *testing.T) {
	updatedAt := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	baseKyc := model.Kyc{
		ApplicantId:          "sumsub-applicant",
		VerificationProvider: model.VerificationProviderSumsub,
		KycStatus:            model.StatusApproved,
		IsActive:             true,
		LastUpdated:          updatedAt,
	}
	baseEvent := model.SumsubEvent{
		ApplicantID: "sumsub-applicant",
		Type:        model.ApplicantReviewed,
		CreatedAtMs: "2026-07-29 11:00:00.000",
	}

	green := baseEvent
	green.ReviewResult.ReviewAnswer = "GREEN"
	result, changed, err := grandfatheredSumsubMonitoringTransition(green, baseKyc)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, model.StatusApproved, result.KycStatus)

	finalRed := baseEvent
	finalRed.ReviewResult.ReviewAnswer = "RED"
	finalRed.ReviewResult.ReviewRejectType = "FINAL"
	result, changed, err = grandfatheredSumsubMonitoringTransition(finalRed, baseKyc)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.StatusFinalRejected, result.KycStatus)

	reset := baseEvent
	reset.Type = model.ApplicantReset
	result, changed, err = grandfatheredSumsubMonitoringTransition(reset, baseKyc)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, model.StatusApproved, result.KycStatus)

	diditOwned := baseKyc
	diditOwned.VerificationProvider = model.VerificationProviderDidit
	result, changed, err = grandfatheredSumsubMonitoringTransition(finalRed, diditOwned)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, model.StatusApproved, result.KycStatus)
}

func appendDiditAnswers(decision *model.DiditDecision, answers map[uuid.UUID]string) {
	items := make([]model.DiditQuestionnaireResponseItem, 0, len(answers))
	for id, value := range answers {
		answer := value
		items = append(items, model.DiditQuestionnaireResponseItem{
			Uuid:   id,
			Answer: &model.DiditQuestionnaireResponseAnswer{Value: &answer},
		})
	}
	decision.QuestionnaireResponses[0].Sections = append(
		decision.QuestionnaireResponses[0].Sections,
		model.DiditQuestionnaireSection{Items: items},
	)
}
