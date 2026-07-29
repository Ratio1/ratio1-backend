package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
)

const (
	defaultDiditTimeout      = 15 * time.Second
	defaultDiditMaxBodyBytes = int64(8 << 20)
)

var (
	ErrDiditResponseTooLarge    = errors.New("didit response exceeds limit")
	ErrDiditVendorMismatch      = errors.New("didit vendor data mismatch")
	ErrDiditWorkflowMismatch    = errors.New("didit workflow mismatch")
	ErrDiditSessionMismatch     = errors.New("didit session mismatch")
	ErrDiditKindMismatch        = errors.New("didit session kind mismatch")
	ErrDiditEnvironmentMismatch = errors.New("didit environment mismatch")
	ErrDiditInvalidResponse     = errors.New("invalid didit response")
)

type DiditAPIError struct {
	Operation   string
	StatusCode  int
	Detail      string
	FieldErrors map[string][]string
	RetryAfter  time.Duration
}

func (e *DiditAPIError) Error() string {
	return fmt.Sprintf("didit %s failed with status %d", e.Operation, e.StatusCode)
}

func (e *DiditAPIError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= http.StatusInternalServerError
}

type DiditClient struct {
	baseURL       *url.URL
	apiKey        string
	environment   string
	workflowKinds map[uuid.UUID]model.DiditSessionKind
	httpClient    *http.Client
	maxBodyBytes  int64
}

func NewDiditClient(cfg config.DiditConfig, httpClient *http.Client) (*DiditClient, error) {
	return newDiditClient(cfg, httpClient, defaultDiditMaxBodyBytes)
}

func newDiditClient(cfg config.DiditConfig, httpClient *http.Client, maxBodyBytes int64) (*DiditClient, error) {
	if strings.TrimSpace(cfg.ApiKey) == "" {
		return nil, errors.New("DIDIT_API_KEY is not set")
	}
	if maxBodyBytes <= 0 {
		return nil, errors.New("didit response limit must be positive")
	}

	baseURL, err := url.Parse(strings.TrimSpace(cfg.ApiUrl))
	if err != nil {
		return nil, errors.New("DIDIT_API_URL is invalid")
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, errors.New("DIDIT_API_URL must use http or https")
	}
	if baseURL.Host == "" || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, errors.New("DIDIT_API_URL is invalid")
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/"
	if baseURL.Scheme != "https" && !isLoopbackHost(baseURL.Hostname()) {
		return nil, errors.New("DIDIT_API_URL must use https outside loopback tests")
	}

	apiEnvironment, err := diditAPIEnvironment(cfg.Environment)
	if err != nil {
		return nil, err
	}
	workflowKinds, err := diditWorkflowKinds(cfg)
	if err != nil {
		return nil, err
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultDiditTimeout}
	}
	httpClientCopy := *httpClient
	if httpClientCopy.Timeout <= 0 {
		httpClientCopy.Timeout = defaultDiditTimeout
	}
	httpClientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &DiditClient{
		baseURL:       baseURL,
		apiKey:        strings.TrimSpace(cfg.ApiKey),
		environment:   apiEnvironment,
		workflowKinds: workflowKinds,
		httpClient:    &httpClientCopy,
		maxBodyBytes:  maxBodyBytes,
	}, nil
}

func (c *DiditClient) CreateSession(ctx context.Context, request model.DiditCreateSessionRequest) (*model.DiditCreateSessionResponse, error) {
	if request.WorkflowId == uuid.Nil {
		return nil, errors.New("didit workflow ID is required")
	}
	if _, err := uuid.Parse(request.VendorData); err != nil {
		return nil, errors.New("didit vendor data must be the KYC UUID")
	}
	if !isKnownDiditSessionKind(request.ExpectedSessionKind) {
		return nil, errors.New("expected didit session kind is invalid")
	}
	if configuredKind, found := c.workflowKinds[request.WorkflowId]; !found || configuredKind != request.ExpectedSessionKind {
		return nil, ErrDiditKindMismatch
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, errors.New("could not encode didit create-session request")
	}

	body, err := c.doJSON(ctx, http.MethodPost, "v3/session/", requestBody, http.StatusCreated, "create session")
	if err != nil {
		return nil, err
	}

	var response model.DiditCreateSessionResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("%w: create-session JSON", ErrDiditInvalidResponse)
	}
	if err = validateDiditCreateSessionResponse(response, request); err != nil {
		return nil, err
	}
	if response.SessionKind == "" {
		response.SessionKind = request.ExpectedSessionKind
	}

	return &response, nil
}

func (c *DiditClient) RetrieveDecision(
	ctx context.Context,
	sessionId uuid.UUID,
	expected model.DiditDecisionExpectation,
) (*model.DiditDecision, error) {
	if sessionId == uuid.Nil {
		return nil, errors.New("didit session ID is required")
	}
	if _, err := uuid.Parse(expected.VendorData); err != nil {
		return nil, errors.New("expected didit vendor data must be the KYC UUID")
	}
	if expected.WorkflowId == uuid.Nil {
		return nil, errors.New("expected didit workflow ID is required")
	}
	if !isKnownDiditSessionKind(expected.SessionKind) {
		return nil, errors.New("expected didit session kind is invalid")
	}
	if configuredKind, found := c.workflowKinds[expected.WorkflowId]; !found || configuredKind != expected.SessionKind {
		return nil, ErrDiditKindMismatch
	}

	path := "v3/session/" + url.PathEscape(sessionId.String()) + "/decision/"
	body, err := c.doJSON(ctx, http.MethodGet, path, nil, http.StatusOK, "retrieve decision")
	if err != nil {
		return nil, err
	}

	var decision model.DiditDecision
	if err = json.Unmarshal(body, &decision); err != nil {
		return nil, fmt.Errorf("%w: decision JSON", ErrDiditInvalidResponse)
	}
	if err = validateDiditDecision(decision, sessionId, expected, c.environment); err != nil {
		return nil, err
	}

	return &decision, nil
}

func (c *DiditClient) RetrieveEntity(
	ctx context.Context,
	sessionKind model.DiditSessionKind,
	vendorData string,
) (*model.DiditEntity, error) {
	if _, err := uuid.Parse(vendorData); err != nil {
		return nil, errors.New("Didit entity vendor data must be the KYC UUID")
	}
	var resource string
	switch sessionKind {
	case model.DiditSessionKindUser:
		resource = "users"
	case model.DiditSessionKindBusiness:
		resource = "businesses"
	default:
		return nil, errors.New("Didit entity session kind is invalid")
	}

	path := "v3/" + resource + "/" + url.PathEscape(vendorData) + "/"
	body, err := c.doJSON(ctx, http.MethodGet, path, nil, http.StatusOK, "retrieve entity")
	if err != nil {
		return nil, err
	}

	var entity model.DiditEntity
	if err = json.Unmarshal(body, &entity); err != nil {
		return nil, fmt.Errorf("%w: entity JSON", ErrDiditInvalidResponse)
	}
	if entity.DiditInternalId == uuid.Nil ||
		entity.VendorData != vendorData ||
		strings.TrimSpace(entity.Status) == "" {
		return nil, ErrDiditInvalidResponse
	}
	return &entity, nil
}

func (c *DiditClient) doJSON(
	ctx context.Context,
	method string,
	path string,
	body []byte,
	expectedStatus int,
	operation string,
) ([]byte, error) {
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: path})
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return nil, errors.New("could not create didit request")
	}
	request.Header.Set("x-api-key", c.apiKey)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("didit %s transport failed: %w", operation, err)
	}
	defer response.Body.Close()

	responseBody, err := readDiditResponseBody(response, c.maxBodyBytes)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != expectedStatus {
		return nil, newDiditAPIError(operation, response, responseBody)
	}

	return responseBody, nil
}

func readDiditResponseBody(response *http.Response, maxBodyBytes int64) ([]byte, error) {
	if response.ContentLength > maxBodyBytes {
		return nil, ErrDiditResponseTooLarge
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return nil, errors.New("could not read didit response")
	}
	if int64(len(body)) > maxBodyBytes {
		return nil, ErrDiditResponseTooLarge
	}

	return body, nil
}

func newDiditAPIError(operation string, response *http.Response, body []byte) *DiditAPIError {
	apiError := &DiditAPIError{
		Operation:   operation,
		StatusCode:  response.StatusCode,
		FieldErrors: make(map[string][]string),
		RetryAfter:  parseDiditRetryAfter(response.Header.Get("Retry-After"), time.Now()),
	}

	var fields map[string]interface{}
	if json.Unmarshal(body, &fields) != nil {
		return apiError
	}
	for field, value := range fields {
		switch typed := value.(type) {
		case string:
			if field == "detail" {
				apiError.Detail = typed
			} else {
				apiError.FieldErrors[field] = []string{typed}
			}
		case []interface{}:
			messages := make([]string, 0, len(typed))
			for _, item := range typed {
				if message, ok := item.(string); ok {
					messages = append(messages, message)
				}
			}
			if len(messages) > 0 {
				apiError.FieldErrors[field] = messages
			}
		}
	}

	return apiError
}

func parseDiditRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil && retryAt.After(now) {
		return retryAt.Sub(now)
	}
	return 0
}

func validateDiditCreateSessionResponse(
	response model.DiditCreateSessionResponse,
	request model.DiditCreateSessionRequest,
) error {
	if response.SessionId == uuid.Nil ||
		response.SessionNumber <= 0 ||
		response.SessionToken == "" ||
		response.Url == "" ||
		response.WorkflowVersion <= 0 ||
		!isAllowedDiditCreateStatus(response.Status) {
		return ErrDiditInvalidResponse
	}
	if response.VendorData != request.VendorData {
		return ErrDiditVendorMismatch
	}
	if response.WorkflowId != request.WorkflowId {
		return ErrDiditWorkflowMismatch
	}
	if response.SessionKind != "" && response.SessionKind != request.ExpectedSessionKind {
		return ErrDiditKindMismatch
	}
	if !isAllowedDiditHostedURL(response.Url) {
		return ErrDiditInvalidResponse
	}
	return nil
}

func isAllowedDiditHostedURL(value string) bool {
	sessionURL, err := url.Parse(strings.TrimSpace(value))
	if err != nil ||
		sessionURL.Scheme != "https" ||
		sessionURL.Host != "verify.didit.me" ||
		sessionURL.User != nil ||
		sessionURL.Fragment != "" {
		return false
	}
	token := strings.TrimPrefix(sessionURL.EscapedPath(), "/session/")
	return token != "" && token != sessionURL.EscapedPath() && !strings.Contains(token, "/")
}

func validateDiditDecision(
	decision model.DiditDecision,
	sessionId uuid.UUID,
	expected model.DiditDecisionExpectation,
	expectedEnvironment string,
) error {
	if decision.SessionId == uuid.Nil ||
		!isKnownDiditSessionKind(decision.SessionKind) ||
		!isKnownDiditSessionStatus(decision.Status) {
		return ErrDiditInvalidResponse
	}
	if decision.SessionId != sessionId {
		return ErrDiditSessionMismatch
	}
	if decision.VendorData != expected.VendorData {
		return ErrDiditVendorMismatch
	}
	if decision.WorkflowId != expected.WorkflowId {
		return ErrDiditWorkflowMismatch
	}
	if decision.SessionKind != expected.SessionKind {
		return ErrDiditKindMismatch
	}
	if decision.Environment != expectedEnvironment {
		return ErrDiditEnvironmentMismatch
	}
	return nil
}

func diditAPIEnvironment(environment string) (string, error) {
	switch environment {
	case model.VerificationEnvironmentSandbox:
		return "sandbox", nil
	case model.VerificationEnvironmentProduction:
		return "live", nil
	default:
		return "", errors.New("didit environment must be sandbox or production")
	}
}

func diditWorkflowKinds(cfg config.DiditConfig) (map[uuid.UUID]model.DiditSessionKind, error) {
	kycWorkflowId, err := uuid.Parse(cfg.KycWorkflowId)
	if err != nil {
		return nil, errors.New("DIDIT_KYC_WORKFLOW_ID is invalid")
	}
	kybWorkflowId, err := uuid.Parse(cfg.KybWorkflowId)
	if err != nil {
		return nil, errors.New("DIDIT_KYB_WORKFLOW_ID is invalid")
	}
	if kycWorkflowId == kybWorkflowId {
		return nil, errors.New("Didit KYC and KYB workflow IDs must differ")
	}
	return map[uuid.UUID]model.DiditSessionKind{
		kycWorkflowId: model.DiditSessionKindUser,
		kybWorkflowId: model.DiditSessionKindBusiness,
	}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isKnownDiditSessionKind(kind model.DiditSessionKind) bool {
	return kind == model.DiditSessionKindUser || kind == model.DiditSessionKindBusiness
}

func isKnownDiditSessionStatus(status model.DiditSessionStatus) bool {
	switch status {
	case model.DiditStatusNotStarted,
		model.DiditStatusInProgress,
		model.DiditStatusAwaitingUser,
		model.DiditStatusInReview,
		model.DiditStatusApproved,
		model.DiditStatusDeclined,
		model.DiditStatusResubmitted,
		model.DiditStatusExpired,
		model.DiditStatusKycExpired,
		model.DiditStatusAbandoned:
		return true
	default:
		return false
	}
}

func isAllowedDiditCreateStatus(status model.DiditSessionStatus) bool {
	switch status {
	case model.DiditStatusNotStarted,
		model.DiditStatusInProgress,
		model.DiditStatusAwaitingUser,
		model.DiditStatusResubmitted:
		return true
	default:
		return false
	}
}
