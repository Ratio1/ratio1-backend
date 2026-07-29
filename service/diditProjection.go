package service

import (
	"encoding/json"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
)

type DiditEntityStatus string

const (
	DiditEntityActive  DiditEntityStatus = "ACTIVE"
	DiditEntityFlagged DiditEntityStatus = "FLAGGED"
	DiditEntityBlocked DiditEntityStatus = "BLOCKED"
)

type DiditEvidenceVerdict uint8

const (
	DiditEvidenceUnknown DiditEvidenceVerdict = iota
	DiditEvidenceIncomplete
	DiditEvidenceEligible
)

type DiditDeclineDisposition uint8

const (
	DiditDeclineUnknown DiditDeclineDisposition = iota
	DiditDeclineRetryable
	DiditDeclineFinal
)

type DiditLifecycleProjectionInput struct {
	SessionStatus      model.DiditSessionStatus
	EntityStatus       DiditEntityStatus
	Evidence           DiditEvidenceVerdict
	DeclineDisposition DiditDeclineDisposition
}

type DiditProjectionReason string

const (
	DiditReasonEntityBlocked           DiditProjectionReason = "entity_blocked"
	DiditReasonEntityFlagged           DiditProjectionReason = "entity_flagged"
	DiditReasonUnknownEntityStatus     DiditProjectionReason = "unknown_entity_status"
	DiditReasonSessionNotStarted       DiditProjectionReason = "session_not_started"
	DiditReasonSessionPending          DiditProjectionReason = "session_pending"
	DiditReasonSessionInReview         DiditProjectionReason = "session_in_review"
	DiditReasonSessionNeedsRetry       DiditProjectionReason = "session_needs_retry"
	DiditReasonApprovalEvidenceReady   DiditProjectionReason = "approval_evidence_ready"
	DiditReasonApprovalEvidenceMissing DiditProjectionReason = "approval_evidence_missing"
	DiditReasonApprovalPolicyRejected  DiditProjectionReason = "approval_policy_rejected"
	DiditReasonDeclineRetryable        DiditProjectionReason = "decline_retryable"
	DiditReasonDeclineFinal            DiditProjectionReason = "decline_final"
	DiditReasonDeclineUnclassified     DiditProjectionReason = "decline_unclassified"
	DiditReasonUnknownProviderStatus   DiditProjectionReason = "unknown_provider_status"
)

type DiditLifecycleProjection struct {
	KycStatus string
	Reason    DiditProjectionReason
}

func (projection DiditLifecycleProjection) GrantsAccess() bool {
	return projection.KycStatus == model.StatusApproved
}

type DiditApprovalPolicy struct {
	WorkflowId                       uuid.UUID
	WorkflowVersion                  int
	SessionKind                      model.DiditSessionKind
	RequiredFeatures                 []string
	QuestionnaireId                  uuid.UUID
	QuestionnaireVersion             int
	RequiredQuestionnaireItems       []uuid.UUID
	ResidenceCountryQuestionId       uuid.UUID
	RestrictedResidenceCountries     map[string]struct{}
	RequireCompanyRegistrationNumber bool
}

// EvaluateDiditApprovalEvidence verifies the feature-level results required by
// the Ratio1 KYC and KYB workflows. A provider-level Approved status alone is
// not sufficient to grant access.
func EvaluateDiditApprovalEvidence(
	decision model.DiditDecision,
	workflowVersion int,
	policy DiditApprovalPolicy,
) DiditEvidenceVerdict {
	if !diditApprovalPolicyIsValid(policy) ||
		decision.WorkflowId != policy.WorkflowId ||
		decision.SessionKind != policy.SessionKind ||
		workflowVersion != policy.WorkflowVersion {
		return DiditEvidenceUnknown
	}
	if decision.Status != model.DiditStatusApproved {
		return DiditEvidenceUnknown
	}
	if diditDecisionHasErrorWarning(decision) ||
		!diditHasExactFeatureSet(decision.Features, policy.RequiredFeatures) ||
		!diditRequiredFeatureEvidenceApproved(decision, policy.RequiredFeatures) ||
		!diditQuestionnaireEvidenceComplete(decision, policy) {
		return DiditEvidenceIncomplete
	}

	switch decision.SessionKind {
	case model.DiditSessionKindUser:
		for _, verification := range decision.IdVerifications {
			if strings.TrimSpace(verification.FullName) == "" ||
				strings.TrimSpace(verification.DateOfBirth) == "" ||
				verification.Age == nil ||
				*verification.Age <= 0 ||
				strings.TrimSpace(verification.Nationality) == "" ||
				strings.TrimSpace(verification.IssuingState) == "" ||
				strings.TrimSpace(verification.DocumentType) == "" {
				return DiditEvidenceIncomplete
			}
		}
		for _, verification := range decision.PoaVerifications {
			if strings.TrimSpace(verification.NameOnDocument) == "" ||
				strings.TrimSpace(verification.PoaAddress) == "" ||
				strings.TrimSpace(verification.IssuingState) == "" ||
				strings.TrimSpace(verification.IssueDate) == "" {
				return DiditEvidenceIncomplete
			}
		}
		for _, screening := range decision.AmlScreenings {
			if screening.TotalHits == nil || *screening.TotalHits != 0 {
				return DiditEvidenceIncomplete
			}
		}
	case model.DiditSessionKindBusiness:
		for _, registry := range decision.RegistryChecks {
			if !diditKybCompanyEvidenceComplete(
				registry.Company,
				policy.RequireCompanyRegistrationNumber,
			) {
				return DiditEvidenceIncomplete
			}
		}
		for _, verification := range decision.DocumentVerifications {
			if !diditKybDocumentEvidenceComplete(verification) {
				return DiditEvidenceIncomplete
			}
		}
		for _, check := range decision.KeyPeopleChecks {
			if !diditKybKeyPeopleEvidenceComplete(check) {
				return DiditEvidenceIncomplete
			}
		}
		for _, screening := range decision.AmlScreenings {
			if screening.TotalHits == nil || *screening.TotalHits != 0 {
				return DiditEvidenceIncomplete
			}
		}
	default:
		return DiditEvidenceUnknown
	}

	return DiditEvidenceEligible
}

func diditApprovalPolicyIsValid(policy DiditApprovalPolicy) bool {
	if policy.WorkflowId == uuid.Nil ||
		policy.WorkflowVersion <= 0 ||
		!isKnownDiditSessionKind(policy.SessionKind) ||
		len(policy.RequiredFeatures) == 0 ||
		policy.QuestionnaireId == uuid.Nil ||
		policy.QuestionnaireVersion <= 0 ||
		len(policy.RequiredQuestionnaireItems) == 0 {
		return false
	}
	requiredItems := make(map[uuid.UUID]struct{}, len(policy.RequiredQuestionnaireItems))
	for _, itemId := range policy.RequiredQuestionnaireItems {
		if itemId == uuid.Nil {
			return false
		}
		requiredItems[itemId] = struct{}{}
	}
	if len(requiredItems) != len(policy.RequiredQuestionnaireItems) {
		return false
	}
	for _, feature := range policy.RequiredFeatures {
		if !diditFeatureSupportedForSession(feature, policy.SessionKind) {
			return false
		}
	}
	if policy.ResidenceCountryQuestionId != uuid.Nil {
		if _, required := requiredItems[policy.ResidenceCountryQuestionId]; !required ||
			len(policy.RestrictedResidenceCountries) == 0 {
			return false
		}
	}
	return true
}

func diditHasExactFeatureSet(actual []string, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, feature := range expected {
		normalized := strings.ToUpper(strings.TrimSpace(feature))
		if normalized == "" {
			return false
		}
		expectedSet[normalized] = struct{}{}
	}
	if len(expectedSet) != len(expected) {
		return false
	}
	for _, feature := range actual {
		normalized := strings.ToUpper(strings.TrimSpace(feature))
		if _, found := expectedSet[normalized]; !found {
			return false
		}
		delete(expectedSet, normalized)
	}
	return len(expectedSet) == 0
}

func diditFeatureSupportedForSession(feature string, sessionKind model.DiditSessionKind) bool {
	normalized := strings.ToUpper(strings.TrimSpace(feature))
	switch normalized {
	case "AML", "PHONE", "EMAIL_VERIFICATION", "IP_ANALYSIS", "QUESTIONNAIRE":
		return true
	case "ID_VERIFICATION", "NFC", "LIVENESS", "FACE_MATCH", "POA", "DOCUMENT_AI", "DATABASE_VALIDATION":
		return sessionKind == model.DiditSessionKindUser
	case "KYB_REGISTRY", "KYB_DOCUMENTS", "KYB_KEY_PEOPLE":
		return sessionKind == model.DiditSessionKindBusiness
	default:
		return false
	}
}

func diditRequiredFeatureEvidenceApproved(decision model.DiditDecision, requiredFeatures []string) bool {
	for _, feature := range requiredFeatures {
		switch strings.ToUpper(strings.TrimSpace(feature)) {
		case "ID_VERIFICATION":
			if !diditAllApproved(decision.IdVerifications, func(result model.DiditIdVerification) string { return result.Status }) {
				return false
			}
		case "NFC":
			if !diditAllApproved(decision.NfcVerifications, func(result model.DiditFeatureResult) string { return result.Status }) {
				return false
			}
		case "LIVENESS":
			if !diditAllApproved(decision.LivenessChecks, func(result model.DiditFeatureResult) string { return result.Status }) {
				return false
			}
		case "FACE_MATCH":
			if !diditAllApproved(decision.FaceMatches, func(result model.DiditFeatureResult) string { return result.Status }) {
				return false
			}
		case "POA":
			if !diditAllApproved(decision.PoaVerifications, func(result model.DiditPoaVerification) string { return result.Status }) {
				return false
			}
		case "PHONE":
			if !diditAllApproved(decision.PhoneVerifications, func(result model.DiditFeatureResult) string { return result.Status }) {
				return false
			}
		case "EMAIL_VERIFICATION":
			if !diditAllApproved(decision.EmailVerifications, func(result model.DiditFeatureResult) string { return result.Status }) {
				return false
			}
		case "DOCUMENT_AI":
			if !diditAllApproved(decision.DocumentAiDocuments, func(result model.DiditDocumentAiResult) string { return result.Status }) {
				return false
			}
		case "AML":
			if !diditAllApproved(decision.AmlScreenings, func(result model.DiditAmlScreening) string { return result.Status }) {
				return false
			}
		case "IP_ANALYSIS":
			if !diditAllApproved(decision.IpAnalyses, func(result model.DiditFeatureResult) string { return result.Status }) {
				return false
			}
		case "DATABASE_VALIDATION":
			if !diditAllApproved(decision.DatabaseValidations, func(result model.DiditFeatureResult) string { return result.Status }) {
				return false
			}
		case "QUESTIONNAIRE":
			if !diditAllApproved(decision.QuestionnaireResponses, func(result model.DiditQuestionnaireResponse) string { return result.Status }) {
				return false
			}
		case "KYB_REGISTRY":
			if !diditAllApproved(decision.RegistryChecks, func(result model.DiditRegistryCheck) string { return result.Status }) {
				return false
			}
		case "KYB_DOCUMENTS":
			if !diditAllApproved(decision.DocumentVerifications, func(result model.DiditDocumentVerification) string { return result.Status }) {
				return false
			}
		case "KYB_KEY_PEOPLE":
			if !diditAllApproved(decision.KeyPeopleChecks, func(result model.DiditKeyPeopleCheck) string { return result.Status }) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func diditQuestionnaireEvidenceComplete(decision model.DiditDecision, policy DiditApprovalPolicy) bool {
	if len(decision.QuestionnaireResponses) != 1 {
		return false
	}
	response := decision.QuestionnaireResponses[0]
	if response.QuestionnaireId != policy.QuestionnaireId ||
		response.Version != policy.QuestionnaireVersion ||
		!diditStatusApproved(response.Status) {
		return false
	}

	items := make(map[uuid.UUID]model.DiditQuestionnaireResponseItem)
	for _, section := range response.Sections {
		for _, item := range section.Items {
			if item.Uuid == uuid.Nil {
				return false
			}
			if _, duplicate := items[item.Uuid]; duplicate {
				return false
			}
			items[item.Uuid] = item
		}
	}
	for _, requiredItemId := range policy.RequiredQuestionnaireItems {
		item, found := items[requiredItemId]
		if !found || !diditQuestionnaireAnswerComplete(item.Answer) {
			return false
		}
	}

	if policy.ResidenceCountryQuestionId != uuid.Nil {
		countryItem, found := items[policy.ResidenceCountryQuestionId]
		if !found || countryItem.Answer == nil || countryItem.Answer.Value == nil {
			return false
		}
		country := strings.ToUpper(strings.TrimSpace(*countryItem.Answer.Value))
		if country == "" {
			return false
		}
		if _, restricted := policy.RestrictedResidenceCountries[country]; restricted {
			return false
		}
	}
	return true
}

func diditQuestionnaireAnswerComplete(answer *model.DiditQuestionnaireResponseAnswer) bool {
	if answer == nil {
		return false
	}
	if answer.Value != nil && strings.TrimSpace(*answer.Value) != "" {
		return true
	}
	if answer.Text != nil && strings.TrimSpace(*answer.Text) != "" {
		return true
	}
	for _, file := range answer.Files {
		if strings.TrimSpace(file) != "" {
			return true
		}
	}
	return false
}

func diditQuestionnaireAnswerValue(
	decision model.DiditDecision,
	questionId uuid.UUID,
) (string, bool) {
	if questionId == uuid.Nil {
		return "", false
	}
	for _, response := range decision.QuestionnaireResponses {
		for _, section := range response.Sections {
			for _, item := range section.Items {
				if item.Uuid == questionId &&
					item.Answer != nil &&
					item.Answer.Value != nil {
					value := strings.TrimSpace(*item.Answer.Value)
					return value, value != ""
				}
			}
		}
	}
	return "", false
}

type diditKybCompanyEvidence struct {
	CompanyName        string `json:"company_name"`
	RegistrationNumber string `json:"registration_number"`
	CountryCode        string `json:"country_code"`
}

func diditKybCompanyEvidenceComplete(
	value json.RawMessage,
	requireRegistrationNumber bool,
) bool {
	var company diditKybCompanyEvidence
	if json.Unmarshal(value, &company) != nil ||
		strings.TrimSpace(company.CompanyName) == "" ||
		strings.TrimSpace(company.CountryCode) == "" {
		return false
	}
	return !requireRegistrationNumber || strings.TrimSpace(company.RegistrationNumber) != ""
}

type diditKybDocumentGroupEvidence struct {
	Total    *int `json:"total"`
	Approved int  `json:"approved"`
	Pending  int  `json:"pending"`
	Declined int  `json:"declined"`
	InReview int  `json:"in_review"`
	Other    int  `json:"other"`
	Missing  int  `json:"missing"`
}

func diditKybDocumentEvidenceComplete(verification model.DiditDocumentVerification) bool {
	var items []map[string]json.RawMessage
	var groups map[string]diditKybDocumentGroupEvidence
	if json.Unmarshal(verification.Items, &items) != nil ||
		json.Unmarshal(verification.Groups, &groups) != nil ||
		len(items) == 0 ||
		len(verification.RequiredGroups) == 0 {
		return false
	}
	for _, item := range items {
		if len(item) == 0 {
			return false
		}
	}
	for _, requiredGroup := range verification.RequiredGroups {
		group, found := groups[requiredGroup]
		if !found {
			group, found = groups[strings.ToLower(requiredGroup)]
		}
		if !found ||
			group.Approved <= 0 ||
			(group.Total != nil && (*group.Total <= 0 || group.Approved != *group.Total)) ||
			group.Pending != 0 ||
			group.Declined != 0 ||
			group.InReview != 0 ||
			group.Other != 0 ||
			group.Missing != 0 {
			return false
		}
	}
	return true
}

type diditKybPartyEvidence struct {
	Name                 string            `json:"name"`
	Role                 string            `json:"role"`
	Roles                []json.RawMessage `json:"roles"`
	KycStatus            string            `json:"kyc_status"`
	KycSessionStatus     string            `json:"kyc_session_status"`
	KybSubSessionStatus  string            `json:"kyb_sub_session_status"`
	RequiresVerification bool              `json:"requires_verification"`
}

type diditKybRegistryPeopleEvidence struct {
	Officers         []diditKybPartyEvidence `json:"officers"`
	BeneficialOwners []diditKybPartyEvidence `json:"beneficial_owners"`
}

type diditKybSubmittedPeopleEvidence struct {
	Parties []diditKybPartyEvidence `json:"parties"`
}

func diditKybKeyPeopleEvidenceComplete(check model.DiditKeyPeopleCheck) bool {
	var registry diditKybRegistryPeopleEvidence
	var submitted diditKybSubmittedPeopleEvidence
	if json.Unmarshal(check.Registry, &registry) != nil ||
		json.Unmarshal(check.Submitted, &submitted) != nil {
		return false
	}
	parties := append(registry.Officers, registry.BeneficialOwners...)
	parties = append(parties, submitted.Parties...)
	if len(parties) == 0 {
		return false
	}
	for _, party := range parties {
		if strings.TrimSpace(party.Name) == "" ||
			!diditKybPartyHasRole(party) {
			return false
		}
		status := party.KycSessionStatus
		if strings.TrimSpace(status) == "" {
			status = party.KybSubSessionStatus
		}
		if strings.TrimSpace(status) == "" {
			status = party.KycStatus
		}
		if party.RequiresVerification && !diditStatusApproved(status) {
			return false
		}
	}
	return true
}

func diditKybPartyHasRole(party diditKybPartyEvidence) bool {
	if strings.TrimSpace(party.Role) != "" {
		return true
	}
	for _, rawRole := range party.Roles {
		var roleName string
		if json.Unmarshal(rawRole, &roleName) == nil && strings.TrimSpace(roleName) != "" {
			return true
		}
		var roleObject struct {
			Role string `json:"role"`
		}
		if json.Unmarshal(rawRole, &roleObject) == nil && strings.TrimSpace(roleObject.Role) != "" {
			return true
		}
	}
	return false
}

func diditAllApproved[T any](results []T, status func(T) string) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if !diditStatusApproved(status(result)) {
			return false
		}
	}
	return true
}

func diditStatusApproved(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), string(model.DiditStatusApproved))
}

func diditDecisionHasErrorWarning(decision model.DiditDecision) bool {
	for _, warning := range diditDecisionWarnings(decision) {
		if strings.EqualFold(strings.TrimSpace(warning.LogType), "error") {
			return true
		}
	}
	return false
}

func ProjectDiditLifecycle(input DiditLifecycleProjectionInput) DiditLifecycleProjection {
	switch input.EntityStatus {
	case DiditEntityBlocked:
		return DiditLifecycleProjection{KycStatus: model.StatusFinalRejected, Reason: DiditReasonEntityBlocked}
	case DiditEntityFlagged:
		return DiditLifecycleProjection{KycStatus: model.StatusOnHold, Reason: DiditReasonEntityFlagged}
	case DiditEntityActive, "":
	default:
		return DiditLifecycleProjection{KycStatus: model.StatusOnHold, Reason: DiditReasonUnknownEntityStatus}
	}

	switch input.SessionStatus {
	case model.DiditStatusNotStarted:
		return DiditLifecycleProjection{KycStatus: model.StatusInit, Reason: DiditReasonSessionNotStarted}
	case model.DiditStatusInProgress, model.DiditStatusAwaitingUser:
		return DiditLifecycleProjection{KycStatus: model.StatusPending, Reason: DiditReasonSessionPending}
	case model.DiditStatusInReview:
		return DiditLifecycleProjection{KycStatus: model.StatusOnHold, Reason: DiditReasonSessionInReview}
	case model.DiditStatusResubmitted,
		model.DiditStatusExpired,
		model.DiditStatusAbandoned,
		model.DiditStatusKycExpired:
		return DiditLifecycleProjection{KycStatus: model.StatusRejected, Reason: DiditReasonSessionNeedsRetry}
	case model.DiditStatusApproved:
		if input.DeclineDisposition == DiditDeclineFinal {
			return DiditLifecycleProjection{KycStatus: model.StatusFinalRejected, Reason: DiditReasonApprovalPolicyRejected}
		}
		if input.Evidence == DiditEvidenceEligible {
			return DiditLifecycleProjection{KycStatus: model.StatusApproved, Reason: DiditReasonApprovalEvidenceReady}
		}
		return DiditLifecycleProjection{KycStatus: model.StatusOnHold, Reason: DiditReasonApprovalEvidenceMissing}
	case model.DiditStatusDeclined:
		switch input.DeclineDisposition {
		case DiditDeclineRetryable:
			return DiditLifecycleProjection{KycStatus: model.StatusRejected, Reason: DiditReasonDeclineRetryable}
		case DiditDeclineFinal:
			return DiditLifecycleProjection{KycStatus: model.StatusFinalRejected, Reason: DiditReasonDeclineFinal}
		default:
			return DiditLifecycleProjection{KycStatus: model.StatusOnHold, Reason: DiditReasonDeclineUnclassified}
		}
	default:
		return DiditLifecycleProjection{KycStatus: model.StatusOnHold, Reason: DiditReasonUnknownProviderStatus}
	}
}
