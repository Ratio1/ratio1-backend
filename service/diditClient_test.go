package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	diditTestAPIKey = "didit_test_api_key"
	diditTestToken  = "test-token-only"
)

var (
	diditTestSessionId          = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	diditTestBusinessSessionId  = uuid.MustParse("00000000-0000-4000-8000-000000000102")
	diditTestWorkflowId         = uuid.MustParse("00000000-0000-4000-8000-000000000201")
	diditTestBusinessWorkflowId = uuid.MustParse("00000000-0000-4000-8000-000000000202")
	diditTestVendorId           = "00000000-0000-4000-8000-000000000301"
	diditTestBusinessVendorId   = "00000000-0000-4000-8000-000000000302"
)

func TestDiditClientCreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/v3/session/", request.URL.Path)
		require.Equal(t, diditTestAPIKey, request.Header.Get("x-api-key"))
		require.Equal(t, "application/json", request.Header.Get("Accept"))
		require.Equal(t, "application/json", request.Header.Get("Content-Type"))

		requestBody, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NotContains(t, string(requestBody), "session_kind")
		require.JSONEq(t, `{
			"workflow_id":"00000000-0000-4000-8000-000000000201",
			"vendor_data":"00000000-0000-4000-8000-000000000301",
			"callback":"https://app.invalid/verification-complete",
			"metadata":{"applicant_type":"individual"}
		}`, string(requestBody))

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write(readDiditFixture(t, "create_session_201.json"))
	}))
	defer server.Close()

	client, err := NewDiditClient(diditTestConfig(server.URL), server.Client())
	require.NoError(t, err)

	response, err := client.CreateSession(context.Background(), model.DiditCreateSessionRequest{
		WorkflowId:          diditTestWorkflowId,
		VendorData:          diditTestVendorId,
		ExpectedSessionKind: model.DiditSessionKindUser,
		Callback:            "https://app.invalid/verification-complete",
		Metadata: map[string]interface{}{
			"applicant_type": "individual",
		},
	})
	require.NoError(t, err)
	require.Equal(t, diditTestSessionId, response.SessionId)
	require.Equal(t, diditTestWorkflowId, response.WorkflowId)
	require.Equal(t, model.DiditSessionKindUser, response.SessionKind)
	require.Equal(t, model.DiditStatusNotStarted, response.Status)
	require.Equal(t, diditTestVendorId, response.VendorData)
	require.Equal(t, diditTestToken, response.SessionToken)
}

func TestDiditClientCreateSessionRejectsInvalidRoutingIdentity(t *testing.T) {
	valid := string(readDiditFixture(t, "create_session_201.json"))
	tests := []struct {
		name        string
		response    string
		expectedErr error
	}{
		{
			name:        "vendor mismatch",
			response:    strings.Replace(valid, diditTestVendorId, diditTestBusinessVendorId, 1),
			expectedErr: ErrDiditVendorMismatch,
		},
		{
			name:        "workflow mismatch",
			response:    strings.Replace(valid, diditTestWorkflowId.String(), diditTestBusinessWorkflowId.String(), 1),
			expectedErr: ErrDiditWorkflowMismatch,
		},
		{
			name:        "missing session",
			response:    strings.Replace(valid, diditTestSessionId.String(), uuid.Nil.String(), 1),
			expectedErr: ErrDiditInvalidResponse,
		},
		{
			name:        "unknown status",
			response:    strings.Replace(valid, string(model.DiditStatusNotStarted), "Unexpected", 1),
			expectedErr: ErrDiditInvalidResponse,
		},
		{
			name:        "terminal create status",
			response:    strings.Replace(valid, string(model.DiditStatusNotStarted), string(model.DiditStatusApproved), 1),
			expectedErr: ErrDiditInvalidResponse,
		},
		{
			name:        "response kind mismatch",
			response:    strings.Replace(valid, "{", `{"session_kind":"business",`, 1),
			expectedErr: ErrDiditKindMismatch,
		},
		{
			name:        "insecure hosted URL",
			response:    strings.Replace(valid, "https://verify.didit.me", "http://verify.didit.me", 1),
			expectedErr: ErrDiditInvalidResponse,
		},
		{
			name:        "unexpected hosted URL origin",
			response:    strings.Replace(valid, "verify.didit.me", "attacker.invalid", 1),
			expectedErr: ErrDiditInvalidResponse,
		},
		{
			name:        "unexpected hosted URL path",
			response:    strings.Replace(valid, "/session/", "/other/", 1),
			expectedErr: ErrDiditInvalidResponse,
		},
		{
			name:        "malformed JSON",
			response:    `{`,
			expectedErr: ErrDiditInvalidResponse,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusCreated)
				_, _ = writer.Write([]byte(test.response))
			}))
			defer server.Close()

			client, err := NewDiditClient(diditTestConfig(server.URL), server.Client())
			require.NoError(t, err)

			_, err = client.CreateSession(context.Background(), model.DiditCreateSessionRequest{
				WorkflowId:          diditTestWorkflowId,
				VendorData:          diditTestVendorId,
				ExpectedSessionKind: model.DiditSessionKindUser,
			})
			require.ErrorIs(t, err, test.expectedErr)
		})
	}
}

func TestDiditClientRejectsWorkflowKindMismatchBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()

	client, err := NewDiditClient(diditTestConfig(server.URL), server.Client())
	require.NoError(t, err)

	_, err = client.CreateSession(context.Background(), model.DiditCreateSessionRequest{
		WorkflowId:          diditTestWorkflowId,
		VendorData:          diditTestVendorId,
		ExpectedSessionKind: model.DiditSessionKindBusiness,
	})
	require.ErrorIs(t, err, ErrDiditKindMismatch)

	_, err = client.RetrieveDecision(context.Background(), diditTestSessionId, model.DiditDecisionExpectation{
		VendorData:  diditTestVendorId,
		WorkflowId:  diditTestWorkflowId,
		SessionKind: model.DiditSessionKindBusiness,
	})
	require.ErrorIs(t, err, ErrDiditKindMismatch)
	require.Zero(t, requests)
}

func TestDiditClientCreateSessionReturnsTypedAPIErrors(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          []byte
		retryAfter    string
		expectedRetry time.Duration
		retryable     bool
		expectedField string
	}{
		{
			name:          "invalid workflow",
			status:        http.StatusBadRequest,
			body:          readDiditFixture(t, "error_invalid_workflow_400.json"),
			retryable:     false,
			expectedField: "workflow_id",
		},
		{
			name:      "unauthorized",
			status:    http.StatusUnauthorized,
			body:      []byte(`{"detail":"Unauthorized."}`),
			retryable: false,
		},
		{
			name:      "forbidden",
			status:    http.StatusForbidden,
			body:      []byte(`{"detail":"Permission denied."}`),
			retryable: false,
		},
		{
			name:          "rate limited",
			status:        http.StatusTooManyRequests,
			body:          readDiditFixture(t, "error_rate_limited_429.json"),
			retryAfter:    "42",
			expectedRetry: 42 * time.Second,
			retryable:     true,
		},
		{
			name:      "server error",
			status:    http.StatusServiceUnavailable,
			body:      []byte(`{"detail":"Unavailable."}`),
			retryable: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				if test.retryAfter != "" {
					writer.Header().Set("Retry-After", test.retryAfter)
				}
				writer.WriteHeader(test.status)
				_, _ = writer.Write(test.body)
			}))
			defer server.Close()

			client, err := NewDiditClient(diditTestConfig(server.URL), server.Client())
			require.NoError(t, err)

			_, err = client.CreateSession(context.Background(), model.DiditCreateSessionRequest{
				WorkflowId:          diditTestWorkflowId,
				VendorData:          diditTestVendorId,
				ExpectedSessionKind: model.DiditSessionKindUser,
			})

			var apiError *DiditAPIError
			require.ErrorAs(t, err, &apiError)
			require.Equal(t, test.status, apiError.StatusCode)
			require.Equal(t, test.expectedRetry, apiError.RetryAfter)
			require.Equal(t, test.retryable, apiError.Retryable())
			if test.expectedField != "" {
				require.NotEmpty(t, apiError.FieldErrors[test.expectedField])
			}
			require.NotContains(t, err.Error(), diditTestAPIKey)
			require.NotContains(t, err.Error(), diditTestToken)
		})
	}
}

func TestDiditClientRetrieveDecision(t *testing.T) {
	tests := []struct {
		name                string
		fixture             string
		sessionId           uuid.UUID
		expected            model.DiditDecisionExpectation
		expectedKind        model.DiditSessionKind
		expectedIdChecks    int
		expectedRegistries  int
		expectedPhoneChecks int
		expectedEmailChecks int
		expectedDocumentAI  int
		expectedIPChecks    int
		expectedDBChecks    int
		expectedReviews     int
	}{
		{
			name:      "complete user decision",
			fixture:   "decision_user_approved.json",
			sessionId: diditTestSessionId,
			expected: model.DiditDecisionExpectation{
				VendorData:  diditTestVendorId,
				WorkflowId:  diditTestWorkflowId,
				SessionKind: model.DiditSessionKindUser,
			},
			expectedKind:        model.DiditSessionKindUser,
			expectedIdChecks:    1,
			expectedPhoneChecks: 1,
			expectedEmailChecks: 1,
			expectedDocumentAI:  1,
			expectedIPChecks:    1,
			expectedDBChecks:    1,
			expectedReviews:     1,
		},
		{
			name:      "incomplete approved user parses without granting evidence",
			fixture:   "decision_user_approved_incomplete.json",
			sessionId: diditTestSessionId,
			expected: model.DiditDecisionExpectation{
				VendorData:  diditTestVendorId,
				WorkflowId:  diditTestWorkflowId,
				SessionKind: model.DiditSessionKindUser,
			},
			expectedKind: model.DiditSessionKindUser,
		},
		{
			name:      "complete business decision",
			fixture:   "decision_business_approved.json",
			sessionId: diditTestBusinessSessionId,
			expected: model.DiditDecisionExpectation{
				VendorData:  diditTestBusinessVendorId,
				WorkflowId:  diditTestBusinessWorkflowId,
				SessionKind: model.DiditSessionKindBusiness,
			},
			expectedKind:       model.DiditSessionKindBusiness,
			expectedRegistries: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				require.Equal(t, http.MethodGet, request.Method)
				require.Equal(t, "/v3/session/"+test.sessionId.String()+"/decision/", request.URL.Path)
				require.Equal(t, diditTestAPIKey, request.Header.Get("x-api-key"))
				require.Equal(t, int64(0), request.ContentLength)
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(readDiditFixture(t, test.fixture))
			}))
			defer server.Close()

			client, err := NewDiditClient(diditTestConfig(server.URL), server.Client())
			require.NoError(t, err)

			decision, err := client.RetrieveDecision(context.Background(), test.sessionId, test.expected)
			require.NoError(t, err)
			require.Equal(t, test.expectedKind, decision.SessionKind)
			require.Equal(t, model.DiditStatusApproved, decision.Status)
			require.Len(t, decision.IdVerifications, test.expectedIdChecks)
			require.Len(t, decision.RegistryChecks, test.expectedRegistries)
			require.Len(t, decision.PhoneVerifications, test.expectedPhoneChecks)
			require.Len(t, decision.EmailVerifications, test.expectedEmailChecks)
			require.Len(t, decision.DocumentAiDocuments, test.expectedDocumentAI)
			require.Len(t, decision.IpAnalyses, test.expectedIPChecks)
			require.Len(t, decision.DatabaseValidations, test.expectedDBChecks)
			require.Len(t, decision.Reviews, test.expectedReviews)
		})
	}
}

func TestDiditClientRetrieveDecisionRejectsRoutingMismatches(t *testing.T) {
	valid := string(readDiditFixture(t, "decision_user_approved.json"))
	tests := []struct {
		name        string
		response    string
		expectedErr error
	}{
		{
			name:        "session mismatch",
			response:    strings.Replace(valid, diditTestSessionId.String(), diditTestBusinessSessionId.String(), 1),
			expectedErr: ErrDiditSessionMismatch,
		},
		{
			name:        "vendor mismatch",
			response:    strings.Replace(valid, diditTestVendorId, diditTestBusinessVendorId, 1),
			expectedErr: ErrDiditVendorMismatch,
		},
		{
			name:        "workflow mismatch",
			response:    strings.Replace(valid, diditTestWorkflowId.String(), diditTestBusinessWorkflowId.String(), 1),
			expectedErr: ErrDiditWorkflowMismatch,
		},
		{
			name:        "kind mismatch",
			response:    strings.Replace(valid, `"session_kind": "user"`, `"session_kind": "business"`, 1),
			expectedErr: ErrDiditKindMismatch,
		},
		{
			name:        "unknown kind",
			response:    strings.Replace(valid, `"session_kind": "user"`, `"session_kind": "unknown"`, 1),
			expectedErr: ErrDiditInvalidResponse,
		},
		{
			name:        "environment mismatch",
			response:    strings.Replace(valid, `"environment": "sandbox"`, `"environment": "live"`, 1),
			expectedErr: ErrDiditEnvironmentMismatch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(test.response))
			}))
			defer server.Close()

			client, err := NewDiditClient(diditTestConfig(server.URL), server.Client())
			require.NoError(t, err)

			_, err = client.RetrieveDecision(context.Background(), diditTestSessionId, model.DiditDecisionExpectation{
				VendorData:  diditTestVendorId,
				WorkflowId:  diditTestWorkflowId,
				SessionKind: model.DiditSessionKindUser,
			})
			require.ErrorIs(t, err, test.expectedErr)
		})
	}
}

func TestDiditClientResponseLimitAndCancellation(t *testing.T) {
	t.Run("oversized response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write(bytes.Repeat([]byte("x"), 65))
		}))
		defer server.Close()

		client, err := newDiditClient(diditTestConfig(server.URL), server.Client(), 64)
		require.NoError(t, err)

		_, err = client.CreateSession(context.Background(), model.DiditCreateSessionRequest{
			WorkflowId:          diditTestWorkflowId,
			VendorData:          diditTestVendorId,
			ExpectedSessionKind: model.DiditSessionKindUser,
		})
		require.ErrorIs(t, err, ErrDiditResponseTooLarge)
	})

	t.Run("cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			<-request.Context().Done()
			writer.WriteHeader(http.StatusGatewayTimeout)
		}))
		defer server.Close()

		client, err := NewDiditClient(diditTestConfig(server.URL), server.Client())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.CreateSession(ctx, model.DiditCreateSessionRequest{
			WorkflowId:          diditTestWorkflowId,
			VendorData:          diditTestVendorId,
			ExpectedSessionKind: model.DiditSessionKindUser,
		})
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestDiditClientRejectsRedirectsWithoutForwardingAPIKey(t *testing.T) {
	redirectTargetCalled := false
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		redirectTargetCalled = true
		require.Empty(t, request.Header.Get("x-api-key"))
	}))
	defer redirectTarget.Close()

	redirectSource := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, redirectTarget.URL, http.StatusTemporaryRedirect)
	}))
	defer redirectSource.Close()

	client, err := NewDiditClient(diditTestConfig(redirectSource.URL), redirectSource.Client())
	require.NoError(t, err)

	_, err = client.CreateSession(context.Background(), model.DiditCreateSessionRequest{
		WorkflowId:          diditTestWorkflowId,
		VendorData:          diditTestVendorId,
		ExpectedSessionKind: model.DiditSessionKindUser,
	})
	var apiError *DiditAPIError
	require.ErrorAs(t, err, &apiError)
	require.Equal(t, http.StatusTemporaryRedirect, apiError.StatusCode)
	require.False(t, redirectTargetCalled)
}

func TestDiditClientAlwaysClosesResponseBody(t *testing.T) {
	trackedBody := &trackedReadCloser{
		Reader: bytes.NewBuffer(readDiditFixture(t, "create_session_201.json")),
	}
	httpClient := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode:    http.StatusCreated,
				Body:          trackedBody,
				Header:        make(http.Header),
				ContentLength: -1,
			}, nil
		}),
	}

	client, err := NewDiditClient(diditTestConfig("https://verification.invalid"), httpClient)
	require.NoError(t, err)

	_, err = client.CreateSession(context.Background(), model.DiditCreateSessionRequest{
		WorkflowId:          diditTestWorkflowId,
		VendorData:          diditTestVendorId,
		ExpectedSessionKind: model.DiditSessionKindUser,
	})
	require.NoError(t, err)
	require.True(t, trackedBody.closed)
}

func TestNewDiditClientValidation(t *testing.T) {
	missingKeyConfig := diditTestConfig("https://verification.invalid")
	missingKeyConfig.ApiKey = ""
	_, err := NewDiditClient(missingKeyConfig, nil)
	require.ErrorContains(t, err, "DIDIT_API_KEY")

	_, err = NewDiditClient(diditTestConfig(":::"), nil)
	require.ErrorContains(t, err, "DIDIT_API_URL")

	_, err = NewDiditClient(diditTestConfig("http://verification.invalid"), nil)
	require.ErrorContains(t, err, "https")

	client, err := NewDiditClient(diditTestConfig("https://verification.invalid"), nil)
	require.NoError(t, err)
	require.Equal(t, defaultDiditTimeout, client.httpClient.Timeout)

	noTimeoutClient := &http.Client{}
	client, err = NewDiditClient(diditTestConfig("https://verification.invalid"), noTimeoutClient)
	require.NoError(t, err)
	require.Equal(t, defaultDiditTimeout, client.httpClient.Timeout)
	require.Zero(t, noTimeoutClient.Timeout)
}

func readDiditFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/didit/" + name)
	require.NoError(t, err)
	return body
}

func diditTestConfig(apiUrl string) config.DiditConfig {
	return config.DiditConfig{
		ApiUrl:        apiUrl,
		ApiKey:        diditTestAPIKey,
		Environment:   model.VerificationEnvironmentSandbox,
		KycWorkflowId: diditTestWorkflowId.String(),
		KybWorkflowId: diditTestBusinessWorkflowId.String(),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type trackedReadCloser struct {
	io.Reader
	closed bool
}

func (body *trackedReadCloser) Close() error {
	if body.closed {
		return errors.New("response body closed twice")
	}
	body.closed = true
	return nil
}
