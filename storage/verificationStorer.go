package storage

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrVerificationEventPayloadMismatch = errors.New("verification webhook event id was reused with a different payload")

func CreateVerificationSession(session *model.VerificationSession) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("verification session is nil")
	}
	if session.KycUuid == uuid.Nil ||
		session.ProviderSessionId == "" ||
		session.KycStatus == "" ||
		session.ProviderStatus == "" {
		return errors.New("verification session identity and status fields are required")
	}
	if err := validateVerificationProvider(session.Provider); err != nil {
		return err
	}
	if err := validateVerificationEnvironment(session.Environment); err != nil {
		return err
	}
	if err := validateApplicantType(session.ApplicantType); err != nil {
		return err
	}
	if session.Provider == model.VerificationProviderDidit && session.WorkflowId == "" {
		return errors.New("Didit verification sessions require a workflow id")
	}
	if session.Uuid == uuid.Nil {
		session.Uuid = uuid.New()
	}

	return db.Create(session).Error
}

func UpdateVerificationSession(session *model.VerificationSession) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("verification session is nil")
	}
	if session.Uuid == uuid.Nil ||
		session.KycStatus == "" ||
		session.ProviderStatus == "" {
		return errors.New("verification session uuid and status fields are required")
	}
	session.UpdatedAt = time.Now().UTC()

	txUpdate := db.Model(&model.VerificationSession{}).
		Where("uuid = ?", session.Uuid).
		Updates(map[string]any{
			"provider_application_id": session.ProviderApplicationId,
			"kyc_status":              session.KycStatus,
			"provider_status":         session.ProviderStatus,
			"decision_at":             session.DecisionAt,
			"last_reconciled_at":      session.LastReconciledAt,
			"updated_at":              session.UpdatedAt,
		})
	if txUpdate.Error != nil {
		return txUpdate.Error
	}
	if txUpdate.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func GetVerificationSession(provider, environment, providerSessionId string) (*model.VerificationSession, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var session model.VerificationSession
	txRead := db.Where(
		"provider = ? AND environment = ? AND provider_session_id = ?",
		provider,
		environment,
		providerSessionId,
	).First(&session)
	if errors.Is(txRead.Error, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}

	return &session, true, nil
}

func GetLatestVerificationSession(
	kycUuid uuid.UUID,
	applicantType, provider, environment string,
) (*model.VerificationSession, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var session model.VerificationSession
	txRead := db.
		Where(
			"kyc_uuid = ? AND applicant_type = ? AND provider = ? AND environment = ?",
			kycUuid,
			applicantType,
			provider,
			environment,
		).
		Order("created_at DESC, uuid DESC").
		First(&session)
	if errors.Is(txRead.Error, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}

	return &session, true, nil
}

func CreateVerificationWebhookEvent(event *model.VerificationWebhookEvent) (bool, error) {
	db, err := GetDB()
	if err != nil {
		return false, err
	}
	if event == nil {
		return false, errors.New("verification webhook event is nil")
	}
	if event.EventId == "" ||
		event.EventType == "" {
		return false, errors.New("verification webhook provider, environment, event id, and event type are required")
	}
	if err := validateVerificationProvider(event.Provider); err != nil {
		return false, err
	}
	if err := validateVerificationEnvironment(event.Environment); err != nil {
		return false, err
	}
	payloadDigest, err := hex.DecodeString(event.PayloadSha256)
	if err != nil || len(payloadDigest) != 32 {
		return false, errors.New("verification webhook payload sha256 must be a 64-character hexadecimal digest")
	}
	event.PayloadSha256 = hex.EncodeToString(payloadDigest)
	if event.Uuid == uuid.Nil {
		event.Uuid = uuid.New()
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now().UTC()
	}
	if event.ProcessingStatus == "" {
		event.ProcessingStatus = model.VerificationEventReceived
	}
	if err := validateVerificationEventStatus(event.ProcessingStatus); err != nil {
		return false, err
	}

	txCreate := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "provider"},
			{Name: "environment"},
			{Name: "event_id"},
		},
		DoNothing: true,
	}).Create(event)
	if txCreate.Error != nil {
		return false, txCreate.Error
	}
	if txCreate.RowsAffected == 0 {
		var stored model.VerificationWebhookEvent
		txRead := db.Where(
			"provider = ? AND environment = ? AND event_id = ?",
			event.Provider,
			event.Environment,
			event.EventId,
		).First(&stored)
		if txRead.Error != nil {
			return false, txRead.Error
		}
		if stored.PayloadSha256 != event.PayloadSha256 {
			return false, ErrVerificationEventPayloadMismatch
		}
	}

	return txCreate.RowsAffected == 1, nil
}

func GetVerificationWebhookEvent(
	provider, environment, eventId string,
) (*model.VerificationWebhookEvent, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var event model.VerificationWebhookEvent
	txRead := db.Where(
		"provider = ? AND environment = ? AND event_id = ?",
		provider,
		environment,
		eventId,
	).First(&event)
	if errors.Is(txRead.Error, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}

	return &event, true, nil
}

func validateVerificationProvider(provider string) error {
	switch provider {
	case model.VerificationProviderSumsub, model.VerificationProviderDidit:
		return nil
	default:
		return fmt.Errorf("unsupported verification provider %q", provider)
	}
}

func validateVerificationEnvironment(environment string) error {
	switch environment {
	case model.VerificationEnvironmentSandbox, model.VerificationEnvironmentProduction:
		return nil
	default:
		return fmt.Errorf("unsupported verification environment %q", environment)
	}
}

func validateApplicantType(applicantType string) error {
	switch applicantType {
	case model.IndividualCustomer, model.BusinessCustomer:
		return nil
	default:
		return fmt.Errorf("unsupported verification applicant type %q", applicantType)
	}
}

func validateVerificationEventStatus(status string) error {
	switch status {
	case model.VerificationEventReceived, model.VerificationEventProcessed, model.VerificationEventFailed:
		return nil
	default:
		return fmt.Errorf("unsupported verification event status %q", status)
	}
}
