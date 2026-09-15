package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOverrideDiditConfigFromEnvironmentMapsWorkflowAndQuestionnaireFields(t *testing.T) {
	t.Setenv("DIDIT_APPLICATION_ID", "  application-id  ")
	t.Setenv("DIDIT_ENVIRONMENT", "  sandbox  ")
	t.Setenv("DIDIT_CALLBACK_URL", "  https://app.example.test/verification  ")
	t.Setenv("DIDIT_KYC_WORKFLOW_VERSION", "7")
	t.Setenv("DIDIT_KYB_WORKFLOW_VERSION", "8")
	t.Setenv("DIDIT_KYC_QUESTIONNAIRE_ID", " kyc-questionnaire ")
	t.Setenv("DIDIT_KYC_QUESTIONNAIRE_VERSION", "9")
	t.Setenv("DIDIT_KYC_FIRST_NAME_QUESTION_ID", " first-name ")
	t.Setenv("DIDIT_KYC_LAST_NAME_QUESTION_ID", " last-name ")
	t.Setenv("DIDIT_KYB_COMPANY_NAME_QUESTION_ID", " company-name ")
	t.Setenv("DIDIT_KYB_TAX_ID_QUESTION_ID", " tax-id ")
	t.Setenv("DIDIT_KYB_ADDRESS_QUESTION_ID", " address ")
	t.Setenv("DIDIT_KYB_ADDITIONAL_REQUIRED_QUESTION_IDS", " first-extra, , second-extra ")

	cfg := DiditConfig{}
	require.NoError(t, overrideDiditConfigFromEnvironment(&cfg))

	require.Equal(t, "application-id", cfg.ApplicationId)
	require.Equal(t, "sandbox", cfg.Environment)
	require.Equal(t, "https://app.example.test/verification", cfg.CallbackUrl)
	require.Equal(t, 7, cfg.KycWorkflowVersion)
	require.Equal(t, 8, cfg.KybWorkflowVersion)
	require.Equal(t, "kyc-questionnaire", cfg.KycQuestionnaire.QuestionnaireId)
	require.Equal(t, 9, cfg.KycQuestionnaire.QuestionnaireVersion)
	require.Equal(t, "first-name", cfg.KycQuestionnaire.FirstNameQuestionId)
	require.Equal(t, "last-name", cfg.KycQuestionnaire.LastNameQuestionId)
	require.Equal(t, "company-name", cfg.KybQuestionnaire.CompanyNameQuestionId)
	require.Equal(t, "tax-id", cfg.KybQuestionnaire.TaxIdQuestionId)
	require.Equal(t, "address", cfg.KybQuestionnaire.AddressQuestionId)
	require.Equal(t, []string{"first-extra", "second-extra"}, cfg.KybQuestionnaire.AdditionalRequiredQuestionIds)
}

func TestOverrideDiditConfigRejectsMalformedOrNonPositiveNumericOverrides(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "DIDIT_KYC_WORKFLOW_VERSION", value: "invalid"},
		{name: "DIDIT_KYB_WORKFLOW_VERSION", value: "0"},
		{name: "DIDIT_KYC_QUESTIONNAIRE_VERSION", value: "-1"},
		{name: "DIDIT_KYB_QUESTIONNAIRE_VERSION", value: "1.5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.name, test.value)

			err := overrideDiditConfigFromEnvironment(&DiditConfig{})
			require.ErrorContains(t, err, test.name+" must be a positive integer")
		})
	}
}

func TestOverrideDiditConfigIgnoresEmptyOverrides(t *testing.T) {
	t.Setenv("DIDIT_APPLICATION_ID", " ")
	t.Setenv("DIDIT_KYB_ADDITIONAL_REQUIRED_QUESTION_IDS", " , ")

	cfg := DiditConfig{
		ApplicationId:      "existing-application",
		KycWorkflowVersion: 3,
		KybWorkflowVersion: 4,
		KycQuestionnaire: DiditQuestionnaireConfig{
			QuestionnaireVersion: 5,
		},
		KybQuestionnaire: DiditQuestionnaireConfig{
			AdditionalRequiredQuestionIds: []string{"existing-extra"},
		},
	}
	require.NoError(t, overrideDiditConfigFromEnvironment(&cfg))

	require.Equal(t, "existing-application", cfg.ApplicationId)
	require.Equal(t, 3, cfg.KycWorkflowVersion)
	require.Equal(t, 4, cfg.KybWorkflowVersion)
	require.Equal(t, 5, cfg.KycQuestionnaire.QuestionnaireVersion)
	require.Equal(t, []string{"existing-extra"}, cfg.KybQuestionnaire.AdditionalRequiredQuestionIds)
}
