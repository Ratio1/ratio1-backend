package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type DiditSessionKind string

const (
	DiditSessionKindUser     DiditSessionKind = "user"
	DiditSessionKindBusiness DiditSessionKind = "business"
)

type DiditSessionStatus string

const (
	DiditStatusNotStarted   DiditSessionStatus = "Not Started"
	DiditStatusInProgress   DiditSessionStatus = "In Progress"
	DiditStatusAwaitingUser DiditSessionStatus = "Awaiting User"
	DiditStatusInReview     DiditSessionStatus = "In Review"
	DiditStatusApproved     DiditSessionStatus = "Approved"
	DiditStatusDeclined     DiditSessionStatus = "Declined"
	DiditStatusResubmitted  DiditSessionStatus = "Resubmitted"
	DiditStatusExpired      DiditSessionStatus = "Expired"
	DiditStatusKycExpired   DiditSessionStatus = "Kyc Expired"
	DiditStatusAbandoned    DiditSessionStatus = "Abandoned"
)

type DiditCreateSessionRequest struct {
	WorkflowId          uuid.UUID              `json:"workflow_id"`
	VendorData          string                 `json:"vendor_data"`
	ExpectedSessionKind DiditSessionKind       `json:"-"`
	Callback            string                 `json:"callback,omitempty"`
	CallbackMethod      string                 `json:"callback_method,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Language            string                 `json:"language,omitempty"`
	ContactDetails      *DiditContactDetails   `json:"contact_details,omitempty"`
	ExpectedDetails     *DiditExpectedDetails  `json:"expected_details,omitempty"`
	SandboxScenario     string                 `json:"sandbox_scenario,omitempty"`
}

type DiditContactDetails struct {
	Email                  string `json:"email,omitempty"`
	SendNotificationEmails bool   `json:"send_notification_emails,omitempty"`
	EmailLang              string `json:"email_lang,omitempty"`
	Phone                  string `json:"phone,omitempty"`
}

type DiditExpectedDetails struct {
	FirstName             string   `json:"first_name,omitempty"`
	LastName              string   `json:"last_name,omitempty"`
	DateOfBirth           string   `json:"date_of_birth,omitempty"`
	Nationality           string   `json:"nationality,omitempty"`
	Country               string   `json:"country,omitempty"`
	IdCountry             string   `json:"id_country,omitempty"`
	PoaCountry            string   `json:"poa_country,omitempty"`
	Address               string   `json:"address,omitempty"`
	IdentificationNumber  string   `json:"identification_number,omitempty"`
	ExpectedDocumentTypes []string `json:"expected_document_types,omitempty"`
	CompanyName           string   `json:"company_name,omitempty"`
	RegistryCountry       string   `json:"registry_country,omitempty"`
	RegistrationNumber    string   `json:"registration_number,omitempty"`
}

type DiditCreateSessionResponse struct {
	SessionId       uuid.UUID          `json:"session_id"`
	SessionKind     DiditSessionKind   `json:"session_kind"`
	SessionNumber   int64              `json:"session_number"`
	SessionToken    string             `json:"session_token"`
	Url             string             `json:"url"`
	VendorData      string             `json:"vendor_data"`
	Metadata        json.RawMessage    `json:"metadata"`
	Status          DiditSessionStatus `json:"status"`
	WorkflowId      uuid.UUID          `json:"workflow_id"`
	WorkflowVersion int                `json:"workflow_version"`
	Callback        *string            `json:"callback"`
}

type DiditDecisionExpectation struct {
	VendorData  string
	WorkflowId  uuid.UUID
	SessionKind DiditSessionKind
}

type DiditDecision struct {
	SessionId              uuid.UUID                    `json:"session_id"`
	SessionKind            DiditSessionKind             `json:"session_kind"`
	SessionNumber          *int64                       `json:"session_number"`
	SessionUrl             string                       `json:"session_url"`
	Status                 DiditSessionStatus           `json:"status"`
	DecisionReasonCode     string                       `json:"decision_reason_code"`
	RiskLevel              string                       `json:"risk_level"`
	OwnershipComplexity    string                       `json:"ownership_complexity"`
	WorkflowId             uuid.UUID                    `json:"workflow_id"`
	VendorData             string                       `json:"vendor_data"`
	Features               []string                     `json:"features"`
	Metadata               json.RawMessage              `json:"metadata"`
	ExpectedDetails        json.RawMessage              `json:"expected_details"`
	ContactDetails         json.RawMessage              `json:"contact_details"`
	Callback               *string                      `json:"callback"`
	CreatedAt              string                       `json:"created_at"`
	ExpiresAt              string                       `json:"expires_at"`
	Environment            string                       `json:"environment"`
	IdVerifications        []DiditIdVerification        `json:"id_verifications"`
	NfcVerifications       []DiditFeatureResult         `json:"nfc_verifications"`
	LivenessChecks         []DiditFeatureResult         `json:"liveness_checks"`
	FaceMatches            []DiditFeatureResult         `json:"face_matches"`
	PoaVerifications       []DiditPoaVerification       `json:"poa_verifications"`
	PhoneVerifications     []DiditFeatureResult         `json:"phone_verifications"`
	EmailVerifications     []DiditFeatureResult         `json:"email_verifications"`
	DocumentAiDocuments    []DiditDocumentAiResult      `json:"document_ai_documents"`
	AmlScreenings          []DiditAmlScreening          `json:"aml_screenings"`
	IpAnalyses             []DiditFeatureResult         `json:"ip_analyses"`
	DatabaseValidations    []DiditFeatureResult         `json:"database_validations"`
	QuestionnaireResponses []DiditQuestionnaireResponse `json:"questionnaire_responses"`
	RegistryChecks         []DiditRegistryCheck         `json:"registry_checks"`
	DocumentVerifications  []DiditDocumentVerification  `json:"document_verifications"`
	KeyPeopleChecks        []DiditKeyPeopleCheck        `json:"key_people_checks"`
	Reviews                []DiditReview                `json:"reviews"`
}

type DiditWarning struct {
	Feature          string          `json:"feature"`
	Risk             string          `json:"risk"`
	AdditionalData   json.RawMessage `json:"additional_data"`
	LogType          string          `json:"log_type"`
	ShortDescription string          `json:"short_description"`
	LongDescription  string          `json:"long_description"`
	NodeId           string          `json:"node_id"`
}

type DiditFeatureResult struct {
	NodeId   string         `json:"node_id"`
	Status   string         `json:"status"`
	Warnings []DiditWarning `json:"warnings"`
}

type DiditIdVerification struct {
	NodeId          string         `json:"node_id"`
	Status          string         `json:"status"`
	FirstName       string         `json:"first_name"`
	LastName        string         `json:"last_name"`
	FullName        string         `json:"full_name"`
	DateOfBirth     string         `json:"date_of_birth"`
	Age             *int           `json:"age"`
	Nationality     string         `json:"nationality"`
	IssuingState    string         `json:"issuing_state"`
	ExpirationDate  string         `json:"expiration_date"`
	DocumentType    string         `json:"document_type"`
	DocumentSubtype string         `json:"document_subtype"`
	Warnings        []DiditWarning `json:"warnings"`
}

type DiditPoaVerification struct {
	NodeId              string         `json:"node_id"`
	Status              string         `json:"status"`
	NameOnDocument      string         `json:"name_on_document"`
	PoaAddress          string         `json:"poa_address"`
	PoaFormattedAddress string         `json:"poa_formatted_address"`
	IssuingState        string         `json:"issuing_state"`
	IssueDate           string         `json:"issue_date"`
	ExpirationDate      string         `json:"expiration_date"`
	Warnings            []DiditWarning `json:"warnings"`
}

type DiditDocumentAiResult struct {
	NodeId   string          `json:"node_id"`
	Status   string          `json:"status"`
	Items    json.RawMessage `json:"items"`
	Warnings []DiditWarning  `json:"warnings"`
}

type DiditAmlScreening struct {
	NodeId                        string          `json:"node_id"`
	Status                        string          `json:"status"`
	TotalHits                     *int            `json:"total_hits"`
	EntityType                    string          `json:"entity_type"`
	Hits                          []DiditAmlHit   `json:"hits"`
	Score                         *float64        `json:"score"`
	ScreenedData                  json.RawMessage `json:"screened_data"`
	IsOngoingMonitoringEnabled    bool            `json:"is_ongoing_monitoring_enabled"`
	NextOngoingMonitoringBillDate *string         `json:"next_ongoing_monitoring_bill_date"`
	Warnings                      []DiditWarning  `json:"warnings"`
}

type DiditAmlHit struct {
	Id                  string          `json:"id"`
	Url                 string          `json:"url"`
	Match               bool            `json:"match"`
	Score               *float64        `json:"score"`
	Target              *bool           `json:"target"`
	Caption             string          `json:"caption"`
	Datasets            []string        `json:"datasets"`
	MatchScore          *float64        `json:"match_score"`
	RiskScore           float64         `json:"risk_score"`
	ReviewStatus        string          `json:"review_status"`
	SanctionMatches     json.RawMessage `json:"sanction_matches"`
	PepMatches          json.RawMessage `json:"pep_matches"`
	WarningMatches      json.RawMessage `json:"warning_matches"`
	AdverseMediaMatches json.RawMessage `json:"adverse_media_matches"`
}

type DiditQuestionnaireResponse struct {
	NodeId               string                      `json:"node_id"`
	Status               string                      `json:"status"`
	QuestionnaireId      uuid.UUID                   `json:"questionnaire_id"`
	QuestionnaireGroupId uuid.UUID                   `json:"questionnaire_group_id"`
	Version              int                         `json:"version"`
	Sections             []DiditQuestionnaireSection `json:"sections"`
}

type DiditQuestionnaireSection struct {
	Title       json.RawMessage                  `json:"title"`
	Description json.RawMessage                  `json:"description"`
	Items       []DiditQuestionnaireResponseItem `json:"items"`
}

type DiditQuestionnaireResponseItem struct {
	Uuid        uuid.UUID                         `json:"uuid"`
	ElementType string                            `json:"element_type"`
	IsRequired  bool                              `json:"is_required"`
	Answer      *DiditQuestionnaireResponseAnswer `json:"answer"`
}

type DiditQuestionnaireResponseAnswer struct {
	Value *string  `json:"value"`
	Text  *string  `json:"text"`
	Files []string `json:"files"`
}

type DiditRegistryCheck struct {
	NodeId             string          `json:"node_id"`
	Status             string          `json:"status"`
	Company            json.RawMessage `json:"company"`
	OwnershipStructure json.RawMessage `json:"ownership_structure"`
	Warnings           []DiditWarning  `json:"warnings"`
}

type DiditDocumentVerification struct {
	NodeId         string          `json:"node_id"`
	Status         string          `json:"status"`
	Items          json.RawMessage `json:"items"`
	Groups         json.RawMessage `json:"groups"`
	RequiredGroups []string        `json:"required_groups"`
	Warnings       []DiditWarning  `json:"warnings"`
}

type DiditKeyPeopleCheck struct {
	NodeId        string          `json:"node_id"`
	Status        string          `json:"status"`
	Registry      json.RawMessage `json:"registry"`
	Submitted     json.RawMessage `json:"submitted"`
	UboKycSummary json.RawMessage `json:"ubo_kyc_summary"`
	Warnings      []DiditWarning  `json:"warnings"`
}

type DiditReview struct {
	Comment   string          `json:"comment"`
	CreatedAt string          `json:"created_at"`
	NewStatus string          `json:"new_status"`
	User      json.RawMessage `json:"user"`
}

type DiditEntity struct {
	DiditInternalId uuid.UUID       `json:"didit_internal_id"`
	VendorData      string          `json:"vendor_data"`
	Status          string          `json:"status"`
	Features        json.RawMessage `json:"features"`
}
