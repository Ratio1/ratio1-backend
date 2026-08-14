package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeVerificationService struct {
	webhookCalls int
	webhookError error
}

func (fake *fakeVerificationService) CreateOrResumeSession(
	context.Context,
	string,
	string,
) (*service.VerificationSessionResponse, error) {
	return nil, nil
}

func (fake *fakeVerificationService) ReceiveDiditWebhook(
	[]byte,
	service.DiditWebhookHeaders,
	time.Time,
) (service.DiditWebhookReceipt, error) {
	fake.webhookCalls++
	return service.DiditWebhookReceipt{}, fake.webhookError
}

func TestDiditWebhookReturnsRetryableStatusForInternalFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeVerificationService{webhookError: errors.New("database unavailable")}
	handler := verificationHandler{service: fake}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/verification/webhooks/didit",
		bytes.NewBufferString(`{}`),
	)

	handler.processDiditWebhook(context)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Equal(t, 1, fake.webhookCalls)
	require.JSONEq(t, `{"accepted":false}`, recorder.Body.String())
}

func TestDiditWebhookReturnsNonRetryableStatusForInvalidEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeVerificationService{webhookError: service.ErrDiditWebhookEnvelope}
	handler := verificationHandler{service: fake}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/verification/webhooks/didit",
		bytes.NewBufferString(`{}`),
	)

	handler.processDiditWebhook(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, 1, fake.webhookCalls)
	require.JSONEq(t, `{"accepted":false}`, recorder.Body.String())
}

func TestDiditWebhookReturnsRetryableStatusWhenBodyReadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeVerificationService{}
	handler := verificationHandler{service: fake}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/verification/webhooks/didit",
		io.NopCloser(errorReader{}),
	)

	handler.processDiditWebhook(context)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Zero(t, fake.webhookCalls)
	require.JSONEq(t, `{"accepted":false}`, recorder.Body.String())
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestDiditWebhookRejectsOversizedBodyBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeVerificationService{}
	handler := verificationHandler{service: fake}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/verification/webhooks/didit",
		bytes.NewReader(make([]byte, maxDiditWebhookBodyBytes+1)),
	)

	handler.processDiditWebhook(context)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Zero(t, fake.webhookCalls)
	require.JSONEq(t, `{"accepted":false}`, recorder.Body.String())
}
