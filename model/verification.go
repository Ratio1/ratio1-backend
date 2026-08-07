package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	VerificationProviderSumsub = "sumsub"
	VerificationProviderDidit  = "didit"

	VerificationEnvironmentSandbox    = "sandbox"
	VerificationEnvironmentProduction = "production"

	VerificationEventReceived   = "received"
	VerificationEventProcessing = "processing"
	VerificationEventProcessed  = "processed"
	VerificationEventFailed     = "failed"
	VerificationEventDeadLetter = "dead_letter"

	VerificationNotificationPending    = "pending"
	VerificationNotificationProcessing = "processing"
	VerificationNotificationSent       = "sent"
	VerificationNotificationFailed     = "failed"

	VerificationNotificationApproved      = "approved"
	VerificationNotificationFinalRejected = "final_rejected"
	VerificationNotificationRetry         = "retry"
)

type VerificationSession struct {
	Uuid                  uuid.UUID  `gorm:"primaryKey;type:text" json:"uuid"`
	KycUuid               uuid.UUID  `gorm:"type:text;not null;index" json:"kyc_uuid"`
	Kyc                   Kyc        `gorm:"foreignKey:KycUuid;references:Uuid;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT" json:"-"`
	Provider              string     `gorm:"type:varchar(32);not null;uniqueIndex:idx_verification_provider_session,priority:1" json:"provider"`
	Environment           string     `gorm:"type:varchar(16);not null;uniqueIndex:idx_verification_provider_session,priority:2" json:"environment"`
	ProviderSessionId     string     `gorm:"type:text;not null;uniqueIndex:idx_verification_provider_session,priority:3" json:"provider_session_id"`
	ProviderApplicationId string     `gorm:"type:text;index" json:"provider_application_id"`
	WorkflowId            string     `gorm:"type:text" json:"workflow_id"`
	WorkflowVersion       string     `gorm:"type:text" json:"workflow_version"`
	ApplicantType         string     `gorm:"type:varchar(16);not null" json:"applicant_type"`
	KycStatus             string     `gorm:"type:varchar(32);not null" json:"kyc_status"`
	ProviderStatus        string     `gorm:"type:text;not null" json:"provider_status"`
	StatusReason          string     `gorm:"type:text" json:"status_reason"`
	DecisionAt            *time.Time `json:"decision_at"`
	LastReconciledAt      *time.Time `json:"last_reconciled_at"`
	CreatedAt             time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"not null" json:"updated_at"`
}

type VerificationWebhookEvent struct {
	Uuid                  uuid.UUID  `gorm:"primaryKey;type:text" json:"uuid"`
	Provider              string     `gorm:"type:varchar(32);not null;uniqueIndex:idx_verification_provider_event,priority:1" json:"provider"`
	Environment           string     `gorm:"type:varchar(16);not null;uniqueIndex:idx_verification_provider_event,priority:2" json:"environment"`
	EventId               string     `gorm:"type:text;not null;uniqueIndex:idx_verification_provider_event,priority:3" json:"event_id"`
	EventType             string     `gorm:"type:text;not null" json:"event_type"`
	ProviderSessionId     string     `gorm:"type:text;index" json:"provider_session_id"`
	ProviderApplicationId string     `gorm:"type:text;index" json:"provider_application_id"`
	VendorData            string     `gorm:"type:text;index" json:"vendor_data"`
	ProviderStatus        string     `gorm:"type:text" json:"provider_status"`
	StatusReason          string     `gorm:"type:text" json:"status_reason"`
	OccurredAt            *time.Time `json:"occurred_at"`
	ReceivedAt            time.Time  `gorm:"not null" json:"received_at"`
	PayloadSha256         string     `gorm:"type:char(64);not null" json:"payload_sha256"`
	ProcessingStatus      string     `gorm:"type:varchar(16);not null" json:"processing_status"`
	Attempts              uint       `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt         *time.Time `gorm:"index" json:"next_attempt_at"`
	ClaimedAt             *time.Time `gorm:"index" json:"claimed_at"`
	ProcessedAt           *time.Time `json:"processed_at"`
	LastError             string     `gorm:"type:varchar(1024)" json:"last_error"`
	UpdatedAt             time.Time  `gorm:"not null" json:"updated_at"`
}

type VerificationNotification struct {
	Uuid                    uuid.UUID  `gorm:"primaryKey;type:text" json:"uuid"`
	VerificationSessionUuid uuid.UUID  `gorm:"type:text;not null;index" json:"verification_session_uuid"`
	TransitionKey           string     `gorm:"type:text;not null;uniqueIndex" json:"transition_key"`
	Email                   string     `gorm:"type:text;not null" json:"email"`
	NotificationType        string     `gorm:"type:varchar(32);not null" json:"notification_type"`
	ProcessingStatus        string     `gorm:"type:varchar(16);not null;index" json:"processing_status"`
	Attempts                uint       `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt           *time.Time `gorm:"index" json:"next_attempt_at"`
	ClaimedAt               *time.Time `gorm:"index" json:"claimed_at"`
	SentAt                  *time.Time `json:"sent_at"`
	LastError               string     `gorm:"type:varchar(1024)" json:"last_error"`
	CreatedAt               time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt               time.Time  `gorm:"not null" json:"updated_at"`
}
