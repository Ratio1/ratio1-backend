package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewDiditPolicySetBuildsRuntimeKycAndKybPolicies(t *testing.T) {
	cfg := diditPolicyTestConfig()

	policies, err := NewDiditPolicySet(cfg)
	require.NoError(t, err)

	require.Equal(t, model.DiditSessionKindUser, policies.Kyc.ApprovalPolicy.SessionKind)
	require.Equal(t, uuid.MustParse(cfg.KycWorkflowId), policies.Kyc.ApprovalPolicy.WorkflowId)
	require.Equal(t, cfg.KycWorkflowVersion, policies.Kyc.ApprovalPolicy.WorkflowVersion)
	require.Equal(t, uuid.MustParse(cfg.KycQuestionnaire.CountryQuestionId), policies.Kyc.ApprovalPolicy.ResidenceCountryQuestionId)
	require.NotEmpty(t, policies.Kyc.ApprovalPolicy.RestrictedResidenceCountries)
	require.ElementsMatch(t, []string{
		"ID_VERIFICATION",
		"LIVENESS",
		"FACE_MATCH",
		"POA",
		"QUESTIONNAIRE",
		"AML",
	}, policies.Kyc.ApprovalPolicy.RequiredFeatures)

	require.Equal(t, model.DiditSessionKindBusiness, policies.Kyb.ApprovalPolicy.SessionKind)
	require.Equal(t, uuid.MustParse(cfg.KybWorkflowId), policies.Kyb.ApprovalPolicy.WorkflowId)
	require.Equal(t, cfg.KybWorkflowVersion, policies.Kyb.ApprovalPolicy.WorkflowVersion)
	require.True(t, policies.Kyb.ApprovalPolicy.RequireCompanyRegistrationNumber)
	require.ElementsMatch(t, []string{
		"KYB_REGISTRY",
		"AML",
		"KYB_DOCUMENTS",
		"KYB_KEY_PEOPLE",
		"QUESTIONNAIRE",
	}, policies.Kyb.ApprovalPolicy.RequiredFeatures)
	require.Contains(
		t,
		policies.Kyb.ApprovalPolicy.RequiredQuestionnaireItems,
		uuid.MustParse(cfg.KybQuestionnaire.AdditionalRequiredQuestionIds[0]),
	)
}

func TestNewDiditPolicySetDoesNotRequireDuplicatedKybBillingQuestions(t *testing.T) {
	cfg := diditPolicyTestConfig()
	cfg.KybQuestionnaire.CompanyNameQuestionId = ""
	cfg.KybQuestionnaire.TaxIdQuestionId = ""
	cfg.KybQuestionnaire.VatNumberQuestionId = ""
	cfg.KybQuestionnaire.AddressQuestionId = ""
	cfg.KybQuestionnaire.CityQuestionId = ""
	cfg.KybQuestionnaire.PostalCodeQuestionId = ""
	cfg.KybQuestionnaire.StateQuestionId = ""
	cfg.KybQuestionnaire.CountryQuestionId = ""

	policies, err := NewDiditPolicySet(cfg)
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, policies.Kyb.QuestionnaireFields.CompanyName)
	require.Len(
		t,
		policies.Kyb.ApprovalPolicy.RequiredQuestionnaireItems,
		len(cfg.KybQuestionnaire.AdditionalRequiredQuestionIds),
	)
}

func TestNewDiditPolicySetRejectsDuplicateQuestionIds(t *testing.T) {
	cfg := diditPolicyTestConfig()
	cfg.KybQuestionnaire.AdditionalRequiredQuestionIds = []string{
		cfg.KybQuestionnaire.AdditionalRequiredQuestionIds[0],
		cfg.KybQuestionnaire.AdditionalRequiredQuestionIds[0],
	}

	_, err := NewDiditPolicySet(cfg)
	require.ErrorContains(t, err, "configured more than once")
}

func TestDiditPolicySetSelectsOnlySupportedApplicantTypes(t *testing.T) {
	policies, err := NewDiditPolicySet(diditPolicyTestConfig())
	require.NoError(t, err)

	kyc, err := policies.ForApplicantType(model.IndividualCustomer)
	require.NoError(t, err)
	require.Equal(t, model.DiditSessionKindUser, kyc.ApprovalPolicy.SessionKind)

	kyb, err := policies.ForApplicantType(model.BusinessCustomer)
	require.NoError(t, err)
	require.Equal(t, model.DiditSessionKindBusiness, kyb.ApprovalPolicy.SessionKind)

	_, err = policies.ForApplicantType("unknown")
	require.ErrorContains(t, err, "unsupported applicant type")
}

func TestMapDiditDecisionToUserInfoFailsClosedWithoutKybTaxOrAddress(t *testing.T) {
	cfg := diditPolicyTestConfig()
	var fixture model.DiditDecision
	require.NoError(t, json.Unmarshal(readDiditFixture(t, "decision_business_approved.json"), &fixture))
	cfg.KybWorkflowId = fixture.WorkflowId.String()
	cfg.KybQuestionnaire.QuestionnaireId = fixture.QuestionnaireResponses[0].QuestionnaireId.String()
	cfg.KybQuestionnaire.QuestionnaireVersion = fixture.QuestionnaireResponses[0].Version

	policies, err := NewDiditPolicySet(cfg)
	require.NoError(t, err)

	validDecision := func(t *testing.T) model.DiditDecision {
		t.Helper()
		var decision model.DiditDecision
		require.NoError(t, json.Unmarshal(readDiditFixture(t, "decision_business_approved.json"), &decision))
		sourceAnswers := make(map[uuid.UUID]string)
		for _, id := range policies.Kyb.ApprovalPolicy.RequiredQuestionnaireItems {
			sourceAnswers[id] = "source-of-funds"
		}
		appendDiditPolicyTestAnswers(&decision, sourceAnswers)
		decision.RegistryChecks[0].Company = json.RawMessage(`{
			"company_name":"Example SRL",
			"country_code":"IT",
			"registration_number":"REG-001",
			"registered_address":"Via Test 1",
			"location_of_registration":"Rome",
			"tax_number":"IT-TAX-001",
			"vat_number":"IT12345678901",
			"vat_validation_status":"valid",
			"user_provided_data":{"region":"RM"}
		}`)
		return decision
	}

	userInfo, country, viesRegistered, err := MapDiditDecisionToUserInfo(
		validDecision(t),
		policies.Kyb,
		"0x0000000000000000000000000000000000000001",
		"company@example.test",
	)
	require.NoError(t, err)
	require.Equal(t, "IT12345678901", userInfo.IdentificationCode)
	require.Equal(t, "Via Test 1", userInfo.Address)
	require.Equal(t, "ITA", country)
	require.True(t, viesRegistered)

	tests := []struct {
		name   string
		mutate func(*model.DiditDecision)
	}{
		{
			name: "missing native tax id",
			mutate: func(decision *model.DiditDecision) {
				decision.RegistryChecks[0].Company = json.RawMessage(`{
					"company_name":"Example SRL",
					"country_code":"ITA",
					"registration_number":"REG-001",
					"registered_address":"Via Test 1",
					"location_of_registration":"Rome",
					"vat_number":"IT12345678901",
					"vat_validation_status":"valid",
					"user_provided_data":{"region":"RM"}
				}`)
			},
		},
		{
			name: "missing native legal address",
			mutate: func(decision *model.DiditDecision) {
				decision.RegistryChecks[0].Company = json.RawMessage(`{
					"company_name":"Example SRL",
					"country_code":"ITA",
					"registration_number":"REG-001",
					"location_of_registration":"Rome",
					"tax_number":"IT-TAX-001",
					"vat_number":"IT12345678901",
					"vat_validation_status":"valid",
					"user_provided_data":{"region":"RM"}
				}`)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := validDecision(t)
			test.mutate(&decision)

			userInfo, country, viesRegistered, err := MapDiditDecisionToUserInfo(
				decision,
				policies.Kyb,
				"0x0000000000000000000000000000000000000001",
				"company@example.test",
			)
			require.Error(t, err)
			require.Nil(t, userInfo)
			require.Empty(t, country)
			require.False(t, viesRegistered)
		})
	}
}

func TestNormalizeDiditCountryCodeUsesUntouchedSandboxKybFixture(t *testing.T) {
	var decision model.DiditDecision
	require.NoError(t, json.Unmarshal(
		readDiditFixture(t, "decision_business_approved.json"),
		&decision,
	))
	company, err := diditApprovedCompany(decision)
	require.NoError(t, err)
	require.Equal(t, "IT", company.CountryCode)

	tests := []struct {
		name     string
		input    string
		expected string
		eu       bool
	}{
		{name: "sandbox EU alpha-2", input: company.CountryCode, expected: "ITA", eu: true},
		{name: "non-EU alpha-2", input: "US", expected: "USA", eu: false},
		{name: "alpha-3 remains canonical", input: "gbr", expected: "GBR", eu: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			country, normalizeErr := normalizeDiditCountryCode(test.input)
			require.NoError(t, normalizeErr)
			require.Equal(t, test.expected, country)
			require.Equal(t, test.eu, isUeCountry(country))
		})
	}

	for _, invalid := range []string{"ZZ", "Italy", "380", "I1", ""} {
		_, err = normalizeDiditCountryCode(invalid)
		require.Error(t, err, invalid)
	}
}

func diditPolicyTestConfig() config.DiditConfig {
	return config.DiditConfig{
		KycWorkflowId:      diditPolicyTestUuid(1).String(),
		KycWorkflowVersion: 3,
		KybWorkflowId:      diditPolicyTestUuid(2).String(),
		KybWorkflowVersion: 4,
		KycQuestionnaire: config.DiditQuestionnaireConfig{
			QuestionnaireId:      diditPolicyTestUuid(10).String(),
			QuestionnaireVersion: 5,
			FirstNameQuestionId:  diditPolicyTestUuid(11).String(),
			LastNameQuestionId:   diditPolicyTestUuid(12).String(),
			TaxIdQuestionId:      diditPolicyTestUuid(13).String(),
			AddressQuestionId:    diditPolicyTestUuid(14).String(),
			CityQuestionId:       diditPolicyTestUuid(15).String(),
			PostalCodeQuestionId: diditPolicyTestUuid(16).String(),
			StateQuestionId:      diditPolicyTestUuid(17).String(),
			CountryQuestionId:    diditPolicyTestUuid(18).String(),
		},
		KybQuestionnaire: config.DiditQuestionnaireConfig{
			QuestionnaireId:      diditPolicyTestUuid(20).String(),
			QuestionnaireVersion: 6,
			AdditionalRequiredQuestionIds: []string{
				diditPolicyTestUuid(29).String(),
				diditPolicyTestUuid(30).String(),
			},
		},
	}
}

func diditPolicyTestUuid(value int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-4000-8000-%012d", value))
}

func appendDiditPolicyTestAnswers(decision *model.DiditDecision, answers map[uuid.UUID]string) {
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
