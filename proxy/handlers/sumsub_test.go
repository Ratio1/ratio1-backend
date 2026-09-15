package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSumsubWebhookRejectsOversizedBodyBeforeDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		baseSumsubEndpoint+hookEndpoint,
		bytes.NewReader(make([]byte, maxSumsubWebhookBodyBytes+1)),
	)

	handler := &sumsubHandler{}
	handler.processEvents(context)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Contains(t, recorder.Body.String(), "invalid Sumsub webhook body")
}
