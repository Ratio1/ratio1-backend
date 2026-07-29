package service

import (
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
)

type DiditDeclinePolicy struct {
	RetryableReasonCodes  map[string]struct{}
	FinalReasonCodes      map[string]struct{}
	RetryableWarningRisks map[string]struct{}
	FinalWarningRisks     map[string]struct{}
}

func DefaultDiditDeclinePolicy() DiditDeclinePolicy {
	return DiditDeclinePolicy{
		RetryableReasonCodes: stringSet(
			"registry_company_not_found",
			"registry_mismatch",
			"documents_failed_ocr",
			"documents_incomplete",
			"key_people_incomplete",
			"timeout_abandoned",
		),
		FinalReasonCodes: stringSet(
			"blocked_country",
			"blocked_business",
			"registry_company_dissolved",
			"aml_confirmed_sanction",
			"aml_confirmed_pep",
			"analyst_rejected",
		),
		RetryableWarningRisks: stringSet(
			"DOCUMENT_EXPIRED",
			"COULD_NOT_RECOGNIZE_DOCUMENT",
			"DOCUMENT_NOT_FULLY_VISIBLE",
			"IMAGE_QUALITY_TOO_LOW",
			"IMAGE_TOO_BLURRY",
			"IMAGE_TOO_BRIGHT",
			"IMAGE_TOO_DARK",
			"NO_FACE_DETECTED",
			"LOW_LIVENESS_SCORE",
			"MISSING_ADDRESS_INFORMATION",
			"POA_DOCUMENT_EXPIRED",
			"INVALID_DOCUMENT_TYPE",
			"UNABLE_TO_VALIDATE_DOCUMENT_AGE",
			"POA_MAX_ATTEMPTS_EXCEEDED",
			"POA_DOCUMENT_NOT_SUPPORTED_FOR_APPLICATION",
			"KYB_DOCUMENT_EXPIRED",
			"KYB_DOCUMENT_MAX_ATTEMPTS_EXCEEDED",
			"KYB_DOCUMENT_MISSING_REQUIRED_FIELD",
		),
		FinalWarningRisks: stringSet(
			"MINIMUM_AGE_NOT_MET",
			"AGE_BELOW_MINIMUM",
			"COUNTRY_NOT_ALLOWED",
			"KYB_COUNTRY_RESTRICTED",
		),
	}
}

func ClassifyDiditDecline(
	decision model.DiditDecision,
	policy DiditDeclinePolicy,
	approvalPolicy DiditApprovalPolicy,
) DiditDeclineDisposition {
	if country, found := diditQuestionnaireAnswerValue(
		decision,
		approvalPolicy.ResidenceCountryQuestionId,
	); found {
		if _, restricted := approvalPolicy.RestrictedResidenceCountries[strings.ToUpper(country)]; restricted {
			return DiditDeclineFinal
		}
	}

	reasonCode := strings.ToLower(strings.TrimSpace(decision.DecisionReasonCode))
	if _, found := policy.FinalReasonCodes[reasonCode]; found {
		return DiditDeclineFinal
	}

	for _, screening := range decision.AmlScreenings {
		if strings.EqualFold(screening.Status, "Declined") &&
			screening.TotalHits != nil &&
			*screening.TotalHits > 0 {
			return DiditDeclineFinal
		}
	}

	warnings := diditDecisionWarnings(decision)
	for _, warning := range warnings {
		risk := strings.ToUpper(strings.TrimSpace(warning.Risk))
		if risk == "" || (warning.LogType != "" && !strings.EqualFold(warning.LogType, "error")) {
			continue
		}
		if _, found := policy.FinalWarningRisks[risk]; found {
			return DiditDeclineFinal
		}
	}

	if decision.Status != model.DiditStatusDeclined {
		return DiditDeclineUnknown
	}

	retryableSignalFound := false
	unknownSignalFound := reasonCode != ""
	if _, found := policy.RetryableReasonCodes[reasonCode]; found {
		retryableSignalFound = true
		unknownSignalFound = false
	}

	for _, warning := range warnings {
		risk := strings.ToUpper(strings.TrimSpace(warning.Risk))
		if risk == "" || (warning.LogType != "" && !strings.EqualFold(warning.LogType, "error")) {
			continue
		}
		if _, found := policy.RetryableWarningRisks[risk]; found {
			retryableSignalFound = true
		} else {
			unknownSignalFound = true
		}
	}

	if unknownSignalFound || !retryableSignalFound {
		return DiditDeclineUnknown
	}
	return DiditDeclineRetryable
}

// Ratio1RestrictedResidenceCountries mirrors the approved Sumsub deny list
// replicated into the current Didit KYC document-country policy.
func Ratio1RestrictedResidenceCountries() map[string]struct{} {
	return stringSet(
		"AFG", "BDI", "BFA", "BLR", "BRB", "CAF", "CCK", "CMR", "COD", "COG",
		"COK", "CXR", "CYM", "EGY", "FLK", "GIN", "GNB", "GRL", "HTI", "IND",
		"IRN", "IRQ", "JAM", "JOR", "KHM", "LBN", "LBY", "LKA", "MAR", "MLI",
		"MMR", "MOZ", "NCL", "NGA", "NIC", "NIU", "PAK", "PAN", "PRK", "PYF",
		"RUS", "SDN", "SEN", "SOM", "SSD", "SYR", "TRN", "TTO", "TUN", "TUR",
		"UGA", "UKR", "USA", "VEN", "VUT", "YEM", "ZWE",
	)
}

func diditDecisionWarnings(decision model.DiditDecision) []model.DiditWarning {
	warnings := make([]model.DiditWarning, 0)
	for _, result := range decision.IdVerifications {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.NfcVerifications {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.LivenessChecks {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.FaceMatches {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.PoaVerifications {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.PhoneVerifications {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.EmailVerifications {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.DocumentAiDocuments {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.AmlScreenings {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.IpAnalyses {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.DatabaseValidations {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.RegistryChecks {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.DocumentVerifications {
		warnings = append(warnings, result.Warnings...)
	}
	for _, result := range decision.KeyPeopleChecks {
		warnings = append(warnings, result.Warnings...)
	}
	return warnings
}

func stringSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
