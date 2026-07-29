package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
)

const verificationClaimLease = 2 * time.Minute

type VerificationWorker struct {
	service *VerificationService
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
}

func (s *VerificationService) StartWorker(parent context.Context) *VerificationWorker {
	worker := &VerificationWorker{service: s, done: make(chan struct{})}
	if s.didit == nil {
		close(worker.done)
		return worker
	}
	ctx, cancel := context.WithCancel(parent)
	worker.cancel = cancel
	go worker.run(ctx)
	return worker
}

func (w *VerificationWorker) Stop() {
	w.once.Do(func() {
		if w.cancel != nil {
			w.cancel()
		}
	})
	<-w.done
}

func (w *VerificationWorker) run(ctx context.Context) {
	defer close(w.done)
	interval := time.Duration(w.service.cfg.Verification.WorkerPollSeconds) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		w.processBatch(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *VerificationWorker) processBatch(ctx context.Context) {
	now := time.Now().UTC()
	batchSize := w.service.cfg.Verification.WorkerBatchSize
	maxAttempts := uint(w.service.cfg.Verification.WorkerMaxAttempts)
	if err := w.service.enqueueStaleDiditSessions(now, batchSize); err != nil {
		log.Error("could not enqueue stale Didit sessions: " + err.Error())
	}
	events, err := storage.ClaimVerificationWebhookEvents(
		now,
		verificationClaimLease,
		batchSize,
		maxAttempts,
	)
	if err != nil {
		log.Error("could not claim verification webhook events: " + err.Error())
		return
	}
	for index := range events {
		event := events[index]
		var processErr error
		switch event.Provider {
		case model.VerificationProviderDidit:
			processErr = w.service.reconcileDiditEvent(ctx, &event)
		case model.VerificationProviderSumsub:
			processErr = w.service.reconcileGrandfatheredSumsubEvent(&event)
		default:
			processErr = errors.New("unsupported verification event provider")
		}
		if processErr != nil {
			nextAttempt := time.Now().UTC().Add(verificationRetryDelay(event.Attempts))
			if markErr := storage.MarkVerificationWebhookEventFailed(
				event.Uuid,
				processErr,
				nextAttempt,
				maxAttempts,
			); markErr != nil {
				log.Error("could not mark verification webhook event failed: " + markErr.Error())
			}
		}
	}
	w.processNotifications(now, batchSize, maxAttempts)
}

func (s *VerificationService) reconcileGrandfatheredSumsubEvent(
	event *model.VerificationWebhookEvent,
) error {
	if s.cfg.Verification.Provider != model.VerificationProviderDidit ||
		!s.cfg.Verification.LegacySumsubWebhooksEnabled {
		return errors.New("grandfathered Sumsub monitoring is inactive")
	}
	if event.OccurredAt == nil ||
		strings.TrimSpace(event.ProviderSessionId) == "" ||
		strings.TrimSpace(event.VendorData) == "" {
		return errors.New("stored Sumsub monitoring event is incomplete")
	}
	kycUuid, err := uuid.Parse(event.VendorData)
	if err != nil {
		return errors.New("stored Sumsub monitoring KYC id is invalid")
	}
	kyc, found, err := storage.GetKycByUuid(kycUuid)
	if err != nil {
		return err
	}
	if !found {
		return ErrorKycNotFound
	}
	sumsubEvent := model.SumsubEvent{
		ApplicantID:    event.ProviderSessionId,
		ExternalUserID: event.VendorData,
		Type:           event.EventType,
		CreatedAtMs:    event.OccurredAt.UTC().Format("2006-01-02 15:04:05.000"),
		ReviewResult: model.ReviewResult{
			ReviewAnswer:     event.ProviderStatus,
			ReviewRejectType: event.StatusReason,
		},
	}
	if err := ProcessGrandfatheredSumsubMonitoringEvent(sumsubEvent, *kyc); err != nil {
		return err
	}
	return storage.MarkVerificationWebhookEventProcessed(event.Uuid)
}

func (s *VerificationService) enqueueStaleDiditSessions(now time.Time, batchSize int) error {
	sessions, err := storage.ListStaleDiditVerificationSessions(
		s.cfg.Didit.Environment,
		now.Add(-5*time.Minute),
		batchSize,
	)
	if err != nil {
		return err
	}
	bucket := now.Unix() / int64((5*time.Minute)/time.Second)
	for index := range sessions {
		session := sessions[index]
		eventId := fmt.Sprintf("sweep:%s:%d", session.ProviderSessionId, bucket)
		digest := sha256.Sum256([]byte(eventId))
		_, createErr := storage.CreateVerificationWebhookEvent(&model.VerificationWebhookEvent{
			Provider:          model.VerificationProviderDidit,
			Environment:       session.Environment,
			EventId:           eventId,
			EventType:         "internal.reconcile",
			ProviderSessionId: session.ProviderSessionId,
			VendorData:        session.KycUuid.String(),
			ReceivedAt:        now,
			PayloadSha256:     hex.EncodeToString(digest[:]),
			ProcessingStatus:  model.VerificationEventReceived,
		})
		if createErr != nil && !errors.Is(createErr, storage.ErrVerificationEventPayloadMismatch) {
			return createErr
		}
	}
	return nil
}

func (s *VerificationService) reconcileDiditEvent(
	ctx context.Context,
	event *model.VerificationWebhookEvent,
) error {
	if event.Provider != model.VerificationProviderDidit {
		return storage.MarkVerificationWebhookEventProcessed(event.Uuid)
	}
	session, err := s.verificationSessionForEvent(event)
	if err != nil {
		return err
	}
	sessionId, err := uuid.Parse(session.ProviderSessionId)
	if err != nil {
		return errors.New("stored Didit session id is invalid")
	}
	if event.VendorData != "" && event.VendorData != session.KycUuid.String() {
		return errors.New("Didit event vendor data does not match stored KYC")
	}
	workflowId, err := uuid.Parse(session.WorkflowId)
	if err != nil {
		return errors.New("stored Didit workflow id is invalid")
	}
	workflowVersion, err := strconv.Atoi(session.WorkflowVersion)
	if err != nil || workflowVersion <= 0 {
		return errors.New("stored Didit workflow version is invalid")
	}
	policy, err := s.diditPolicies.ForApplicantType(session.ApplicantType)
	if err != nil {
		return err
	}
	if policy.ApprovalPolicy.WorkflowId != workflowId ||
		policy.ApprovalPolicy.WorkflowVersion != workflowVersion {
		return errors.New("stored Didit session policy does not match configured workflow")
	}

	reconciliationStartedAt := time.Now().UTC()
	decision, err := s.didit.RetrieveDecision(ctx, sessionId, model.DiditDecisionExpectation{
		VendorData:  session.KycUuid.String(),
		WorkflowId:  workflowId,
		SessionKind: policy.ApprovalPolicy.SessionKind,
	})
	if err != nil {
		return err
	}
	entity, err := s.didit.RetrieveEntity(
		ctx,
		policy.ApprovalPolicy.SessionKind,
		session.KycUuid.String(),
	)
	if err != nil {
		return err
	}
	entityStatus, err := projectableDiditEntityStatus(entity.Status)
	if err != nil {
		return err
	}
	evidence := EvaluateDiditApprovalEvidence(*decision, workflowVersion, policy.ApprovalPolicy)
	decline := ClassifyDiditDecline(*decision, DefaultDiditDeclinePolicy(), policy.ApprovalPolicy)
	projection := ProjectDiditLifecycle(DiditLifecycleProjectionInput{
		SessionStatus:      decision.Status,
		EntityStatus:       entityStatus,
		Evidence:           evidence,
		DeclineDisposition: decline,
	})

	kyc, found, err := storage.GetKycByUuid(session.KycUuid)
	if err != nil {
		return err
	}
	if !found {
		return ErrorKycNotFound
	}
	var userInfo *model.UserInfo
	country := ""
	viesRegistered := false
	if projection.KycStatus == model.StatusApproved {
		account, found, accountErr := storage.GetAccountByEmail(kyc.Email)
		if accountErr != nil {
			return accountErr
		}
		if !found {
			return ErrorAccountNotFound
		}
		userInfo, country, viesRegistered, err = MapDiditDecisionToUserInfo(
			*decision,
			policy,
			account.Address,
			kyc.Email,
		)
		if err != nil {
			return err
		}
	}

	var decisionAt *time.Time
	if diditDecisionIsTerminal(decision.Status) {
		terminalAt := reconciliationStartedAt
		decisionAt = &terminalAt
	}
	notificationType := diditNotificationType(projection.KycStatus)
	_, err = storage.ApplyVerificationProjection(storage.VerificationProjectionUpdate{
		EventUuid:                 event.Uuid,
		SessionUuid:               session.Uuid,
		KycStatus:                 projection.KycStatus,
		ProviderStatus:            string(decision.Status),
		StatusReason:              string(projection.Reason),
		DecisionAt:                decisionAt,
		ReconciledAt:              reconciliationStartedAt,
		Country:                   country,
		ViesRegistered:            viesRegistered,
		UserInfo:                  userInfo,
		NotificationType:          notificationType,
		NotificationTransitionKey: session.Uuid.String() + ":" + projection.KycStatus,
	})
	return err
}

func (s *VerificationService) verificationSessionForEvent(
	event *model.VerificationWebhookEvent,
) (*model.VerificationSession, error) {
	if event.ProviderSessionId != "" {
		session, found, err := storage.GetVerificationSession(
			model.VerificationProviderDidit,
			event.Environment,
			event.ProviderSessionId,
		)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, errors.New("Didit webhook references an unknown session")
		}
		return session, nil
	}
	kycUuid, err := uuid.Parse(event.VendorData)
	if err != nil {
		return nil, errors.New("Didit entity webhook vendor data is invalid")
	}
	session, found, err := storage.GetLatestVerificationSessionForKyc(
		kycUuid,
		model.VerificationProviderDidit,
		event.Environment,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("Didit entity webhook has no local verification session")
	}
	return session, nil
}

func (w *VerificationWorker) processNotifications(
	now time.Time,
	batchSize int,
	maxAttempts uint,
) {
	notifications, err := storage.ClaimVerificationNotifications(
		now,
		verificationClaimLease,
		batchSize,
		maxAttempts,
	)
	if err != nil {
		log.Error("could not claim verification notifications: " + err.Error())
		return
	}
	for index := range notifications {
		notification := notifications[index]
		var sendErr error
		switch notification.NotificationType {
		case model.VerificationNotificationApproved:
			sendErr = SendKycConfirmedEmail(notification.Email)
		case model.VerificationNotificationFinalRejected:
			sendErr = SendKycFinalRejectedEmail(notification.Email)
		case model.VerificationNotificationRetry:
			sendErr = SendStepRejectedEmail(notification.Email)
		default:
			sendErr = fmt.Errorf("unsupported verification notification type %q", notification.NotificationType)
		}
		nextAttempt := time.Now().UTC().Add(verificationRetryDelay(notification.Attempts))
		if err := storage.CompleteVerificationNotification(
			notification.Uuid,
			sendErr,
			nextAttempt,
			maxAttempts,
		); err != nil {
			log.Error("could not complete verification notification: " + err.Error())
		}
	}
}

func projectableDiditEntityStatus(value string) (DiditEntityStatus, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ACTIVE", "APPROVED":
		return DiditEntityActive, nil
	case "FLAGGED", "IN REVIEW", "PENDING":
		return DiditEntityFlagged, nil
	case "BLOCKED", "DECLINED":
		return DiditEntityBlocked, nil
	default:
		return "", errors.New("unknown Didit entity status")
	}
}

func diditDecisionIsTerminal(status model.DiditSessionStatus) bool {
	switch status {
	case model.DiditStatusApproved,
		model.DiditStatusDeclined,
		model.DiditStatusExpired,
		model.DiditStatusKycExpired,
		model.DiditStatusAbandoned:
		return true
	default:
		return false
	}
}

func diditNotificationType(status string) string {
	switch status {
	case model.StatusApproved:
		return model.VerificationNotificationApproved
	case model.StatusFinalRejected:
		return model.VerificationNotificationFinalRejected
	case model.StatusRejected:
		return model.VerificationNotificationRetry
	default:
		return ""
	}
}

func verificationRetryDelay(attempt uint) time.Duration {
	if attempt == 0 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<uint(attempt-1)) * time.Minute
}
