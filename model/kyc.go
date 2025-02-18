package model

import (
	"time"

	"github.com/google/uuid"
)

type Kyc struct {
	Uuid           uuid.UUID `gorm:"primarykey;unique" json:"uuid"`
	ApplicantId    string    `json:"applicant_id"`
	ApplicantType  string    `json:"applicant_type"`
	Email          string    `json:"email"`
	KycStatus      string    `json:"kyc_status"`
	LastUpdated    time.Time `json:"last_updated"`
	IsActive       bool      `gorm:"not null;default:true" json:"is_active"`
	HasBeenDeleted bool      `gorm:"not null;default:false" json:"has_been_deleted"`
	ReceiveUpdates *bool     `gorm:"not null;default:false" json:"receiveUpdates"`
}

const (
	ApplicantCreated               string = "applicantCreated"
	ApplicantPending               string = "applicantPending"
	ApplicantReviewed              string = "applicantReviewed"
	ApplicantOnHold                string = "applicantOnHold"
	ApplicantActionPending         string = "applicantActionPending"
	ApplicantActionReviewed        string = "applicantActionReviewed"
	ApplicantActionOnHold          string = "applicantActionOnHold"
	ApplicantPersonalInfoChanged   string = "applicantPersonalInfoChanged"
	ApplicantTagsChanged           string = "applicantTagsChanged"
	ApplicantActivated             string = "applicantActivated"
	ApplicantDeactivated           string = "applicantDeactivated"
	ApplicantDeleted               string = "applicantDeleted"
	ApplicantReset                 string = "applicantReset"
	ApplicantPrechecked            string = "applicantPrechecked"
	ApplicantLevelChanged          string = "applicantLevelChanged"
	ApplicantWorkflowCompleted     string = "applicantWorkflowCompleted"
	VideoIdentStatusChanged        string = "videoIdentStatusChanged"
	VideoIdentCompositionCompleted string = "videoIdentCompositionCompleted"

	StatusAccountCreated = "accCreated"
	StatusInit           = "init"
	StatusPending        = "pending"
	StatusPrechecked     = "prechecked"
	StatusQueued         = "queued"
	StatusCompleted      = "completed"
	StatusApproved       = "approved"
	StatusOnHold         = "onHold"
	StatusRejected       = "rejected"
	StatusFinalRejected  = "finalRejected"

	IndividualCustomer = "individual"
	BusinessCustomer   = "company"
)
