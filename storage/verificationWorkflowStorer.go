package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VerificationProjectionUpdate struct {
	EventUuid                 uuid.UUID
	SessionUuid               uuid.UUID
	KycStatus                 string
	ProviderStatus            string
	StatusReason              string
	DecisionAt                *time.Time
	ReconciledAt              time.Time
	Country                   string
	ViesRegistered            bool
	UserInfo                  *model.UserInfo
	NotificationType          string
	NotificationTransitionKey string
}

func WithVerificationCreationLock(
	ctx context.Context,
	kycUuid uuid.UUID,
	provider, environment string,
	action func() error,
) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	sqlDb, err := db.DB()
	if err != nil {
		return err
	}
	connection, err := sqlDb.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()

	lockKey := strings.Join([]string{
		"verification-session",
		kycUuid.String(),
		provider,
		environment,
	}, ":")
	if _, err := connection.ExecContext(ctx, "SELECT pg_advisory_lock(hashtext($1))", lockKey); err != nil {
		return err
	}
	defer func() {
		_, _ = connection.ExecContext(context.Background(), "SELECT pg_advisory_unlock(hashtext($1))", lockKey)
	}()
	return action()
}

func AssignVerificationSession(session *model.VerificationSession) (*model.VerificationSession, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.New("verification session is nil")
	}
	if err := validateVerificationSession(session); err != nil {
		return nil, err
	}

	var stored model.VerificationSession
	err = db.Transaction(func(tx *gorm.DB) error {
		var kyc model.Kyc
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uuid = ?", session.KycUuid).
			First(&kyc).Error; err != nil {
			return err
		}
		if kyc.VerificationProvider != "" && kyc.VerificationProvider != session.Provider {
			return fmt.Errorf(
				"kyc is owned by verification provider %q",
				kyc.VerificationProvider,
			)
		}
		if kyc.ApplicantType != "" && kyc.ApplicantType != session.ApplicantType {
			return errors.New("kyc applicant type cannot be changed after verification starts")
		}

		if session.Uuid == uuid.Nil {
			session.Uuid = uuid.New()
		}
		now := time.Now().UTC()
		if session.CreatedAt.IsZero() {
			session.CreatedAt = now
		}
		session.UpdatedAt = now

		create := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "provider"},
				{Name: "environment"},
				{Name: "provider_session_id"},
			},
			DoNothing: true,
		}).Create(session)
		if create.Error != nil {
			return create.Error
		}
		if err := tx.Where(
			"provider = ? AND environment = ? AND provider_session_id = ?",
			session.Provider,
			session.Environment,
			session.ProviderSessionId,
		).First(&stored).Error; err != nil {
			return err
		}
		if stored.KycUuid != session.KycUuid ||
			stored.ApplicantType != session.ApplicantType ||
			stored.WorkflowId != session.WorkflowId {
			return errors.New("existing provider session belongs to a different verification")
		}

		updates := map[string]interface{}{
			"verification_provider": session.Provider,
			"applicant_type":        session.ApplicantType,
		}
		newlyCreatedReplacement := create.RowsAffected == 1 &&
			session.Provider == model.VerificationProviderDidit &&
			kyc.KycStatus == model.StatusRejected &&
			session.KycStatus == model.StatusInit
		if kyc.KycStatus == model.StatusAccountCreated || newlyCreatedReplacement {
			updates["kyc_status"] = session.KycStatus
			updates["last_updated"] = now
		}
		return tx.Model(&model.Kyc{}).
			Where("uuid = ?", session.KycUuid).
			Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return &stored, nil
}

func ClaimVerificationWebhookEvents(
	now time.Time,
	leaseDuration time.Duration,
	batchSize int,
	maxAttempts uint,
) ([]model.VerificationWebhookEvent, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	if batchSize <= 0 || maxAttempts == 0 {
		return nil, errors.New("verification event claim limits must be positive")
	}
	leaseExpiredAt := now.Add(-leaseDuration)
	claimed := make([]model.VerificationWebhookEvent, 0, batchSize)

	err = db.Transaction(func(tx *gorm.DB) error {
		var events []model.VerificationWebhookEvent
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(
				`provider IN (?, ?) AND
				attempts < ? AND
				(next_attempt_at IS NULL OR next_attempt_at <= ?) AND
				(
					processing_status IN (?, ?) OR
					(processing_status = ? AND claimed_at < ?)
				)`,
				model.VerificationProviderDidit,
				model.VerificationProviderSumsub,
				maxAttempts,
				now,
				model.VerificationEventReceived,
				model.VerificationEventFailed,
				model.VerificationEventProcessing,
				leaseExpiredAt,
			).
			Order("received_at ASC, uuid ASC").
			Limit(batchSize).
			Find(&events)
		if query.Error != nil {
			return query.Error
		}

		for index := range events {
			event := &events[index]
			update := tx.Model(&model.VerificationWebhookEvent{}).
				Where("uuid = ?", event.Uuid).
				Updates(map[string]interface{}{
					"processing_status": model.VerificationEventProcessing,
					"attempts":          gorm.Expr("attempts + 1"),
					"claimed_at":        now,
					"updated_at":        now,
				})
			if update.Error != nil {
				return update.Error
			}
			event.ProcessingStatus = model.VerificationEventProcessing
			event.Attempts++
			event.ClaimedAt = &now
		}
		claimed = events
		return nil
	})
	return claimed, err
}

func MarkVerificationWebhookEventFailed(
	eventUuid uuid.UUID,
	failure error,
	nextAttemptAt time.Time,
	maxAttempts uint,
) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if failure == nil {
		return errors.New("verification event failure is required")
	}

	var event model.VerificationWebhookEvent
	if err := db.Where("uuid = ?", eventUuid).First(&event).Error; err != nil {
		return err
	}
	status := model.VerificationEventFailed
	var retryAt *time.Time
	if event.Attempts >= maxAttempts {
		status = model.VerificationEventDeadLetter
	} else {
		retryAt = &nextAttemptAt
	}
	now := time.Now().UTC()
	return db.Model(&model.VerificationWebhookEvent{}).
		Where("uuid = ?", eventUuid).
		Updates(map[string]interface{}{
			"processing_status": status,
			"next_attempt_at":   retryAt,
			"claimed_at":        nil,
			"last_error":        truncateVerificationError(failure.Error()),
			"updated_at":        now,
		}).Error
}

func MarkVerificationWebhookEventProcessed(eventUuid uuid.UUID) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return db.Model(&model.VerificationWebhookEvent{}).
		Where("uuid = ?", eventUuid).
		Updates(map[string]interface{}{
			"processing_status": model.VerificationEventProcessed,
			"processed_at":      now,
			"next_attempt_at":   nil,
			"claimed_at":        nil,
			"last_error":        "",
			"updated_at":        now,
		}).Error
}

func MarkVerificationWebhookEventProcessedByIdentity(
	provider, environment, eventId string,
) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	update := db.Model(&model.VerificationWebhookEvent{}).
		Where(
			"provider = ? AND environment = ? AND event_id = ?",
			provider,
			environment,
			eventId,
		).
		Updates(map[string]interface{}{
			"processing_status": model.VerificationEventProcessed,
			"processed_at":      now,
			"next_attempt_at":   nil,
			"claimed_at":        nil,
			"last_error":        "",
			"updated_at":        now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func RetryDeadLetterVerificationWebhookEvent(eventUuid uuid.UUID) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	update := db.Model(&model.VerificationWebhookEvent{}).
		Where("uuid = ? AND processing_status = ?", eventUuid, model.VerificationEventDeadLetter).
		Updates(map[string]interface{}{
			"processing_status": model.VerificationEventFailed,
			"attempts":          0,
			"next_attempt_at":   now,
			"claimed_at":        nil,
			"last_error":        "",
			"updated_at":        now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ClaimVerificationNotifications(
	now time.Time,
	leaseDuration time.Duration,
	batchSize int,
	maxAttempts uint,
) ([]model.VerificationNotification, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	leaseExpiredAt := now.Add(-leaseDuration)
	var claimed []model.VerificationNotification
	err = db.Transaction(func(tx *gorm.DB) error {
		var notifications []model.VerificationNotification
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(
				`attempts < ? AND
				(next_attempt_at IS NULL OR next_attempt_at <= ?) AND
				(
					processing_status IN (?, ?) OR
					(processing_status = ? AND claimed_at < ?)
				)`,
				maxAttempts,
				now,
				model.VerificationNotificationPending,
				model.VerificationNotificationFailed,
				model.VerificationNotificationProcessing,
				leaseExpiredAt,
			).
			Order("created_at ASC, uuid ASC").
			Limit(batchSize).
			Find(&notifications)
		if query.Error != nil {
			return query.Error
		}
		for index := range notifications {
			notification := &notifications[index]
			if err := tx.Model(&model.VerificationNotification{}).
				Where("uuid = ?", notification.Uuid).
				Updates(map[string]interface{}{
					"processing_status": model.VerificationNotificationProcessing,
					"attempts":          gorm.Expr("attempts + 1"),
					"claimed_at":        now,
					"updated_at":        now,
				}).Error; err != nil {
				return err
			}
			notification.ProcessingStatus = model.VerificationNotificationProcessing
			notification.Attempts++
			notification.ClaimedAt = &now
		}
		claimed = notifications
		return nil
	})
	return claimed, err
}

func CompleteVerificationNotification(
	notificationUuid uuid.UUID,
	sendError error,
	nextAttemptAt time.Time,
	maxAttempts uint,
) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	var notification model.VerificationNotification
	if err := db.Where("uuid = ?", notificationUuid).First(&notification).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"claimed_at": nil,
		"updated_at": now,
	}
	if sendError == nil {
		updates["processing_status"] = model.VerificationNotificationSent
		updates["sent_at"] = now
		updates["next_attempt_at"] = nil
		updates["last_error"] = ""
	} else {
		updates["processing_status"] = model.VerificationNotificationFailed
		updates["next_attempt_at"] = nextAttemptAt
		updates["last_error"] = truncateVerificationError(sendError.Error())
		if notification.Attempts >= maxAttempts {
			updates["next_attempt_at"] = nil
		}
	}
	return db.Model(&model.VerificationNotification{}).
		Where("uuid = ?", notificationUuid).
		Updates(updates).Error
}

func ApplyVerificationProjection(update VerificationProjectionUpdate) (bool, error) {
	db, err := GetDB()
	if err != nil {
		return false, err
	}
	if update.EventUuid == uuid.Nil ||
		update.SessionUuid == uuid.Nil ||
		update.ReconciledAt.IsZero() ||
		update.KycStatus == "" ||
		update.ProviderStatus == "" {
		return false, errors.New("verification projection identity and status are required")
	}

	appliedToKyc := false
	err = db.Transaction(func(tx *gorm.DB) error {
		var event model.VerificationWebhookEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uuid = ?", update.EventUuid).
			First(&event).Error; err != nil {
			return err
		}
		if event.ProcessingStatus == model.VerificationEventProcessed {
			return nil
		}

		var session model.VerificationSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uuid = ?", update.SessionUuid).
			First(&session).Error; err != nil {
			return err
		}
		if session.LastReconciledAt != nil &&
			session.LastReconciledAt.After(update.ReconciledAt) {
			return markVerificationEventProcessed(tx, event.Uuid, update.ReconciledAt)
		}

		sessionUpdates := map[string]interface{}{
			"kyc_status":         update.KycStatus,
			"provider_status":    update.ProviderStatus,
			"status_reason":      update.StatusReason,
			"decision_at":        update.DecisionAt,
			"last_reconciled_at": update.ReconciledAt,
			"updated_at":         update.ReconciledAt,
		}
		if err := tx.Model(&model.VerificationSession{}).
			Where("uuid = ?", session.Uuid).
			Updates(sessionUpdates).Error; err != nil {
			return err
		}

		var latest model.VerificationSession
		latestRead := tx.Where(
			"kyc_uuid = ? AND provider = ? AND environment = ? AND applicant_type = ?",
			session.KycUuid,
			session.Provider,
			session.Environment,
			session.ApplicantType,
		).Order("created_at DESC, uuid DESC").First(&latest)
		if latestRead.Error != nil {
			return latestRead.Error
		}

		var kyc model.Kyc
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uuid = ?", session.KycUuid).
			First(&kyc).Error; err != nil {
			return err
		}
		if latest.Uuid == session.Uuid && kyc.VerificationProvider == session.Provider {
			if update.KycStatus == model.StatusApproved && update.UserInfo == nil {
				return errors.New("approved verification requires complete user info")
			}
			kycUpdates := map[string]interface{}{
				"kyc_status":   update.KycStatus,
				"last_updated": update.ReconciledAt,
			}
			if update.Country != "" {
				kycUpdates["country"] = update.Country
				kycUpdates["vies_registered"] = update.ViesRegistered
			}
			if err := tx.Model(&model.Kyc{}).
				Where("uuid = ?", kyc.Uuid).
				Updates(kycUpdates).Error; err != nil {
				return err
			}
			if update.UserInfo != nil {
				if err := createOrUpdateVerificationUserInfo(tx, update.UserInfo); err != nil {
					return err
				}
			}
			if update.NotificationType != "" && kyc.KycStatus != update.KycStatus {
				if err := createVerificationNotification(
					tx,
					session.Uuid,
					kyc.Email,
					update.NotificationType,
					update.NotificationTransitionKey,
					update.ReconciledAt,
				); err != nil {
					return err
				}
			}
			appliedToKyc = true
		}

		return markVerificationEventProcessed(tx, event.Uuid, update.ReconciledAt)
	})
	return appliedToKyc, err
}

func GetLatestVerificationSessionForKyc(
	kycUuid uuid.UUID,
	provider, environment string,
) (*model.VerificationSession, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}
	var session model.VerificationSession
	query := db.Where(
		"kyc_uuid = ? AND provider = ? AND environment = ?",
		kycUuid,
		provider,
		environment,
	).Order("created_at DESC, uuid DESC").First(&session)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return &session, query.Error == nil, query.Error
}

func ListStaleDiditVerificationSessions(
	environment string,
	reconciledBefore time.Time,
	limit int,
) ([]model.VerificationSession, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, errors.New("stale verification session limit must be positive")
	}
	var sessions []model.VerificationSession
	err = db.Where(
		`provider = ? AND environment = ? AND
		provider_status NOT IN (?, ?, ?, ?, ?) AND
		COALESCE(last_reconciled_at, updated_at, created_at) <= ?`,
		model.VerificationProviderDidit,
		environment,
		string(model.DiditStatusApproved),
		string(model.DiditStatusDeclined),
		string(model.DiditStatusExpired),
		string(model.DiditStatusKycExpired),
		string(model.DiditStatusAbandoned),
		reconciledBefore,
	).Order("COALESCE(last_reconciled_at, updated_at, created_at) ASC").
		Limit(limit).
		Find(&sessions).Error
	return sessions, err
}

func createOrUpdateVerificationUserInfo(tx *gorm.DB, userInfo *model.UserInfo) error {
	if strings.TrimSpace(userInfo.BlockchainAddress) == "" {
		return errors.New("verification user info blockchain address is required")
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "blockchain_address"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"email",
			"name",
			"surname",
			"company_name",
			"identification_code",
			"address",
			"state",
			"city",
			"country",
			"is_company",
		}),
	}).Create(userInfo).Error
}

func createVerificationNotification(
	tx *gorm.DB,
	sessionUuid uuid.UUID,
	email, notificationType, transitionKey string,
	now time.Time,
) error {
	if transitionKey == "" {
		transitionKey = sessionUuid.String() + ":" + notificationType
	}
	notification := model.VerificationNotification{
		Uuid:                    uuid.New(),
		VerificationSessionUuid: sessionUuid,
		TransitionKey:           transitionKey,
		Email:                   email,
		NotificationType:        notificationType,
		ProcessingStatus:        model.VerificationNotificationPending,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "transition_key"}},
		DoNothing: true,
	}).Create(&notification).Error
}

func markVerificationEventProcessed(tx *gorm.DB, eventUuid uuid.UUID, now time.Time) error {
	return tx.Model(&model.VerificationWebhookEvent{}).
		Where("uuid = ?", eventUuid).
		Updates(map[string]interface{}{
			"processing_status": model.VerificationEventProcessed,
			"processed_at":      now,
			"next_attempt_at":   nil,
			"claimed_at":        nil,
			"last_error":        "",
			"updated_at":        now,
		}).Error
}

func validateVerificationSession(session *model.VerificationSession) error {
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
	return nil
}

func truncateVerificationError(message string) string {
	const maxLength = 1024
	if len(message) <= maxLength {
		return message
	}
	return message[:maxLength]
}
