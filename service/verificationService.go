package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
)

type VerificationSessionResponse struct {
	Provider      string `json:"provider"`
	ApplicantType string `json:"applicantType"`
	Status        string `json:"status"`
	SessionId     string `json:"sessionId,omitempty"`
	Url           string `json:"url,omitempty"`
	AccessToken   string `json:"accessToken,omitempty"`
}

var ErrVerificationReconciliationPending = errors.New(
	"Didit session is complete and pending authoritative reconciliation",
)

func (response VerificationSessionResponse) Validate() error {
	switch response.Provider {
	case model.VerificationProviderDidit:
		if strings.TrimSpace(response.SessionId) == "" ||
			strings.TrimSpace(response.Url) == "" ||
			strings.TrimSpace(response.AccessToken) != "" {
			return errors.New("Didit session response must contain only sessionId and url credentials")
		}
	case model.VerificationProviderSumsub:
		if strings.TrimSpace(response.AccessToken) == "" ||
			strings.TrimSpace(response.SessionId) != "" ||
			strings.TrimSpace(response.Url) != "" {
			return errors.New("Sumsub session response must contain only an accessToken credential")
		}
	default:
		return errors.New("verification session response has an unsupported provider")
	}
	if response.ApplicantType != model.IndividualCustomer &&
		response.ApplicantType != model.BusinessCustomer {
		return errors.New("verification session response has an unsupported applicant type")
	}
	if strings.TrimSpace(response.Status) == "" {
		return errors.New("verification session response status is required")
	}
	return nil
}

type diditSessionClient interface {
	CreateSession(context.Context, model.DiditCreateSessionRequest) (*model.DiditCreateSessionResponse, error)
	RetrieveDecision(context.Context, uuid.UUID, model.DiditDecisionExpectation) (*model.DiditDecision, error)
	RetrieveEntity(context.Context, model.DiditSessionKind, string) (*model.DiditEntity, error)
}

type VerificationService struct {
	cfg               config.GeneralConfig
	didit             diditSessionClient
	diditPolicies     DiditPolicySet
	sumsubInitSession func(string, string) (*string, error)
}

type DiditWebhookHeaders struct {
	Timestamp   string
	SignatureV2 string
	Signature   string
	Simple      string
	TestWebhook bool
}

type DiditWebhookReceipt struct {
	Duplicate bool
	TestOnly  bool
}

type diditWebhookPayload struct {
	EventId       uuid.UUID              `json:"event_id"`
	WebhookType   string                 `json:"webhook_type"`
	Timestamp     int64                  `json:"timestamp"`
	CreatedAt     int64                  `json:"created_at"`
	ApplicationId uuid.UUID              `json:"application_id"`
	Environment   string                 `json:"environment"`
	SessionId     uuid.UUID              `json:"session_id"`
	SessionKind   model.DiditSessionKind `json:"session_kind"`
	VendorData    string                 `json:"vendor_data"`
	Status        string                 `json:"status"`
}

var supportedDiditVerificationEvents = map[string]struct{}{
	"status.updated":          {},
	"data.updated":            {},
	"user.status.updated":     {},
	"user.data.updated":       {},
	"business.status.updated": {},
	"business.data.updated":   {},
}

func NewVerificationService(cfg config.GeneralConfig) (*VerificationService, error) {
	service := &VerificationService{
		cfg:               cfg,
		sumsubInitSession: InitNewSession,
	}
	diditConfigured := strings.TrimSpace(cfg.Didit.ApiKey) != "" ||
		strings.TrimSpace(cfg.Didit.WebhookSecret) != "" ||
		strings.TrimSpace(cfg.Didit.ApplicationId) != ""
	if cfg.Verification.Provider == model.VerificationProviderDidit || diditConfigured {
		client, err := NewDiditClient(cfg.Didit, nil)
		if err != nil {
			return nil, err
		}
		policies, err := NewDiditPolicySet(cfg.Didit)
		if err != nil {
			return nil, err
		}
		if _, err := uuid.Parse(cfg.Didit.ApplicationId); err != nil {
			return nil, errors.New("DIDIT_APPLICATION_ID is invalid")
		}
		if strings.TrimSpace(cfg.Didit.WebhookSecret) == "" {
			return nil, errors.New("DIDIT_WEBHOOK_SECRET is not set")
		}
		if err := validateDiditCallbackUrl(cfg.Didit.CallbackUrl); err != nil {
			return nil, err
		}
		service.didit = client
		service.diditPolicies = policies
	}
	switch cfg.Verification.Provider {
	case model.VerificationProviderSumsub, model.VerificationProviderDidit:
		return service, nil
	default:
		return nil, errors.New("unsupported verification provider")
	}
}

func (s *VerificationService) ReceiveDiditWebhook(
	body []byte,
	headers DiditWebhookHeaders,
	now time.Time,
) (DiditWebhookReceipt, error) {
	if s.didit == nil {
		return DiditWebhookReceipt{}, errors.New("Didit integration is inactive")
	}
	signatureHeaders := DiditWebhookSignatureHeaders{
		Timestamp: headers.Timestamp,
		V2:        headers.SignatureV2,
		Raw:       headers.Signature,
		Simple:    headers.Simple,
	}
	_, signatureErr := VerifyDiditWebhookSignatures(
		body,
		signatureHeaders,
		s.cfg.Didit.WebhookSecret,
		now,
	)
	if signatureErr != nil && strings.TrimSpace(s.cfg.Didit.PreviousWebhookSecret) != "" {
		_, signatureErr = VerifyDiditWebhookSignatures(
			body,
			signatureHeaders,
			s.cfg.Didit.PreviousWebhookSecret,
			now,
		)
	}
	if signatureErr != nil {
		return DiditWebhookReceipt{}, signatureErr
	}

	var payload diditWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return DiditWebhookReceipt{}, ErrDiditWebhookEnvelope
	}
	if headers.TestWebhook {
		if s.cfg.Didit.Environment != model.VerificationEnvironmentSandbox {
			return DiditWebhookReceipt{}, errors.New("Didit test webhooks are disabled outside sandbox")
		}
		return DiditWebhookReceipt{TestOnly: true}, nil
	}
	if payload.EventId == uuid.Nil ||
		payload.ApplicationId == uuid.Nil ||
		strings.TrimSpace(payload.VendorData) == "" ||
		payload.Timestamp == 0 ||
		payload.CreatedAt == 0 {
		return DiditWebhookReceipt{}, ErrDiditWebhookEnvelope
	}
	if _, allowed := supportedDiditVerificationEvents[payload.WebhookType]; !allowed {
		return DiditWebhookReceipt{}, errors.New("unsupported Didit verification event")
	}
	expectedApplicationId, _ := uuid.Parse(s.cfg.Didit.ApplicationId)
	if payload.ApplicationId != expectedApplicationId {
		return DiditWebhookReceipt{}, ErrDiditEnvironmentMismatch
	}
	environment, err := internalDiditEnvironment(payload.Environment)
	if err != nil || environment != s.cfg.Didit.Environment {
		return DiditWebhookReceipt{}, ErrDiditEnvironmentMismatch
	}
	if _, err := uuid.Parse(payload.VendorData); err != nil {
		return DiditWebhookReceipt{}, ErrDiditWebhookEnvelope
	}
	if isDiditSessionEvent(payload.WebhookType) && payload.SessionId == uuid.Nil {
		return DiditWebhookReceipt{}, ErrDiditWebhookEnvelope
	}
	expectedKind := diditSessionKindForEvent(payload.WebhookType)
	if expectedKind != "" {
		if payload.SessionKind != "" && payload.SessionKind != expectedKind {
			return DiditWebhookReceipt{}, ErrDiditWebhookEnvelope
		}
		payload.SessionKind = expectedKind
	} else if payload.SessionKind != "" &&
		payload.SessionKind != model.DiditSessionKindUser &&
		payload.SessionKind != model.DiditSessionKindBusiness {
		return DiditWebhookReceipt{}, ErrDiditWebhookEnvelope
	}

	digest := sha256.Sum256(body)
	occurredAt := time.Unix(payload.CreatedAt, 0).UTC()
	created, err := storage.CreateVerificationWebhookEvent(&model.VerificationWebhookEvent{
		Provider:              model.VerificationProviderDidit,
		Environment:           environment,
		EventId:               payload.EventId.String(),
		EventType:             payload.WebhookType,
		ProviderSessionId:     nilUuidString(payload.SessionId),
		ProviderApplicationId: payload.ApplicationId.String(),
		VendorData:            payload.VendorData,
		OccurredAt:            &occurredAt,
		ReceivedAt:            now.UTC(),
		PayloadSha256:         hex.EncodeToString(digest[:]),
		ProcessingStatus:      model.VerificationEventReceived,
	})
	if err != nil {
		return DiditWebhookReceipt{}, err
	}
	return DiditWebhookReceipt{Duplicate: !created}, nil
}

func (s *VerificationService) CreateOrResumeSession(
	ctx context.Context,
	address, applicantType string,
) (*VerificationSessionResponse, error) {
	if applicantType != model.IndividualCustomer && applicantType != model.BusinessCustomer {
		return nil, errors.New("type must be individual or company")
	}
	account, err := GetOrCreateAccount(address)
	if err != nil {
		return nil, err
	}
	if account == nil || account.Email == nil || !account.EmailConfirmed {
		return nil, errors.New("email is not confirmed")
	}
	if account.IsBlacklisted {
		return nil, errors.New("account is blacklisted")
	}
	kyc, found, err := storage.GetKycByEmail(*account.Email)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrorKycNotFound
	}
	if kyc.KycStatus == model.StatusApproved {
		return nil, errors.New("verification is already approved")
	}
	if kyc.KycStatus == model.StatusFinalRejected {
		return nil, errors.New("verification is final rejected and cannot be retried")
	}
	if kyc.ApplicantType != "" && kyc.ApplicantType != applicantType {
		return nil, errors.New("verification type cannot be changed after verification starts")
	}
	if kyc.VerificationProvider != "" &&
		kyc.VerificationProvider != s.cfg.Verification.Provider {
		return nil, fmt.Errorf(
			"verification is owned by provider %q and requires an explicit cutover",
			kyc.VerificationProvider,
		)
	}

	switch s.cfg.Verification.Provider {
	case model.VerificationProviderSumsub:
		return s.createSumsubSession(kyc, applicantType)
	case model.VerificationProviderDidit:
		return s.createOrResumeDiditSession(ctx, kyc, applicantType)
	default:
		return nil, errors.New("unsupported verification provider")
	}
}

func (s *VerificationService) createSumsubSession(
	kyc *model.Kyc,
	applicantType string,
) (*VerificationSessionResponse, error) {
	level := s.cfg.Sumsub.CustomerLevelName
	if applicantType == model.BusinessCustomer {
		level = s.cfg.Sumsub.BusinessLevelName
	}
	token, err := s.sumsubInitSession(kyc.Uuid.String(), level)
	if err != nil {
		return nil, err
	}
	if token == nil || strings.TrimSpace(*token) == "" {
		return nil, errors.New("Sumsub returned an empty access token")
	}
	kyc.ApplicantType = applicantType
	kyc.VerificationProvider = model.VerificationProviderSumsub
	if kyc.KycStatus == model.StatusAccountCreated {
		kyc.KycStatus = model.StatusInit
		kyc.LastUpdated = time.Now().UTC()
	}
	if err := storage.CreateOrUpdateKyc(kyc); err != nil {
		return nil, err
	}
	result := &VerificationSessionResponse{
		Provider:      model.VerificationProviderSumsub,
		ApplicantType: applicantType,
		Status:        kyc.KycStatus,
		AccessToken:   *token,
	}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *VerificationService) createOrResumeDiditSession(
	ctx context.Context,
	kyc *model.Kyc,
	applicantType string,
) (*VerificationSessionResponse, error) {
	if s.didit == nil {
		return nil, errors.New("Didit integration is unavailable")
	}
	policy, err := s.diditPolicies.ForApplicantType(applicantType)
	if err != nil {
		return nil, err
	}

	var result *VerificationSessionResponse
	err = storage.WithVerificationCreationLock(
		ctx,
		kyc.Uuid,
		model.VerificationProviderDidit,
		s.cfg.Didit.Environment,
		func() error {
			var lockedErr error
			result, lockedErr = s.createOrResumeDiditSessionLocked(ctx, kyc, applicantType, policy)
			return lockedErr
		},
	)
	return result, err
}

func (s *VerificationService) createOrResumeDiditSessionLocked(
	ctx context.Context,
	kyc *model.Kyc,
	applicantType string,
	policy DiditVerificationPolicy,
) (*VerificationSessionResponse, error) {
	latest, found, err := storage.GetLatestVerificationSession(
		kyc.Uuid,
		applicantType,
		model.VerificationProviderDidit,
		s.cfg.Didit.Environment,
	)
	if err != nil {
		return nil, err
	}
	if found {
		sessionId, parseErr := uuid.Parse(latest.ProviderSessionId)
		if parseErr != nil {
			return nil, errors.New("stored Didit session id is invalid")
		}
		decision, retrieveErr := s.didit.RetrieveDecision(ctx, sessionId, model.DiditDecisionExpectation{
			VendorData:  kyc.Uuid.String(),
			WorkflowId:  policy.ApprovalPolicy.WorkflowId,
			SessionKind: policy.ApprovalPolicy.SessionKind,
		})
		if retrieveErr != nil {
			return nil, retrieveErr
		}
		if diditSessionCanResume(decision.Status) && isAllowedDiditHostedURL(decision.SessionUrl) {
			result := &VerificationSessionResponse{
				Provider:      model.VerificationProviderDidit,
				ApplicantType: applicantType,
				Status:        latest.KycStatus,
				SessionId:     sessionId.String(),
				Url:           decision.SessionUrl,
			}
			if err := result.Validate(); err != nil {
				return nil, err
			}
			return result, nil
		}
		if diditSessionCanResume(decision.Status) {
			return nil, errors.New("Didit returned an invalid hosted session URL")
		}
		if err := enqueueDiditPollReconciliation(latest, decision); err != nil {
			return nil, err
		}
		reconciledKyc, kycFound, readErr := storage.GetKycByUuid(kyc.Uuid)
		if readErr != nil {
			return nil, readErr
		}
		if !kycFound {
			return nil, ErrorKycNotFound
		}
		switch reconciledKyc.KycStatus {
		case model.StatusApproved:
			return nil, errors.New("verification is already approved")
		case model.StatusFinalRejected:
			return nil, errors.New("verification is final rejected and cannot be retried")
		}
		if !diditTerminalSessionWasReconciledAsRetryable(latest, decision, reconciledKyc) {
			return nil, ErrVerificationReconciliationPending
		}
		kyc = reconciledKyc
	}

	response, err := s.didit.CreateSession(ctx, diditCreateSessionRequest(
		s.cfg.Didit.CallbackUrl,
		kyc,
		applicantType,
		policy,
	))
	if err != nil {
		return nil, err
	}
	session, err := storage.AssignVerificationSession(&model.VerificationSession{
		KycUuid:           kyc.Uuid,
		Provider:          model.VerificationProviderDidit,
		Environment:       s.cfg.Didit.Environment,
		ProviderSessionId: response.SessionId.String(),
		WorkflowId:        response.WorkflowId.String(),
		WorkflowVersion:   fmt.Sprintf("%d", response.WorkflowVersion),
		ApplicantType:     applicantType,
		KycStatus:         model.StatusInit,
		ProviderStatus:    string(response.Status),
	})
	if err != nil {
		return nil, err
	}
	result := &VerificationSessionResponse{
		Provider:      model.VerificationProviderDidit,
		ApplicantType: applicantType,
		Status:        session.KycStatus,
		SessionId:     response.SessionId.String(),
		Url:           response.Url,
	}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}

func diditTerminalSessionWasReconciledAsRetryable(
	session *model.VerificationSession,
	decision *model.DiditDecision,
	kyc *model.Kyc,
) bool {
	return session.LastReconciledAt != nil &&
		session.KycStatus == model.StatusRejected &&
		kyc.KycStatus == model.StatusRejected &&
		diditDecisionIsTerminal(decision.Status) &&
		session.ProviderStatus == string(decision.Status)
}

func diditCreateSessionRequest(
	callbackUrl string,
	kyc *model.Kyc,
	applicantType string,
	policy DiditVerificationPolicy,
) model.DiditCreateSessionRequest {
	return model.DiditCreateSessionRequest{
		WorkflowId:          policy.ApprovalPolicy.WorkflowId,
		VendorData:          kyc.Uuid.String(),
		ExpectedSessionKind: policy.ApprovalPolicy.SessionKind,
		Callback:            callbackUrl,
		CallbackMethod:      "initiator",
		Metadata: map[string]interface{}{
			"ratio1_applicant_type": applicantType,
		},
		ContactDetails: &model.DiditContactDetails{
			Email:                  kyc.Email,
			SendNotificationEmails: false,
		},
	}
}

func diditSessionCanResume(status model.DiditSessionStatus) bool {
	switch status {
	case model.DiditStatusNotStarted,
		model.DiditStatusInProgress,
		model.DiditStatusAwaitingUser,
		model.DiditStatusInReview,
		model.DiditStatusResubmitted:
		return true
	default:
		return false
	}
}

func enqueueDiditPollReconciliation(
	session *model.VerificationSession,
	decision *model.DiditDecision,
) error {
	payload, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	eventId := "poll:" +
		session.ProviderSessionId + ":" +
		strings.ToLower(strings.ReplaceAll(string(decision.Status), " ", "_")) + ":" +
		hex.EncodeToString(digest[:8])
	_, err = storage.CreateVerificationWebhookEvent(&model.VerificationWebhookEvent{
		Provider:          model.VerificationProviderDidit,
		Environment:       session.Environment,
		EventId:           eventId,
		EventType:         "internal.reconcile",
		ProviderSessionId: session.ProviderSessionId,
		VendorData:        decision.VendorData,
		ReceivedAt:        time.Now().UTC(),
		PayloadSha256:     hex.EncodeToString(digest[:]),
		ProcessingStatus:  model.VerificationEventReceived,
	})
	return err
}

func validateDiditCallbackUrl(value string) error {
	value = strings.TrimSpace(value)
	callback, err := url.Parse(value)
	if err != nil ||
		callback.Scheme != "https" ||
		callback.Host == "" ||
		callback.User != nil {
		return errors.New("DIDIT_CALLBACK_URL must use https")
	}
	return nil
}

func internalDiditEnvironment(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sandbox":
		return model.VerificationEnvironmentSandbox, nil
	case "live", "production":
		return model.VerificationEnvironmentProduction, nil
	default:
		return "", errors.New("unsupported Didit webhook environment")
	}
}

func isDiditSessionEvent(eventType string) bool {
	return eventType == "status.updated" || eventType == "data.updated"
}

func diditSessionKindForEvent(eventType string) model.DiditSessionKind {
	switch {
	case strings.HasPrefix(eventType, "user."):
		return model.DiditSessionKindUser
	case strings.HasPrefix(eventType, "business."):
		return model.DiditSessionKindBusiness
	default:
		return ""
	}
}

func nilUuidString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
