package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/gin-gonic/gin"
)

const (
	baseVerificationEndpoint    = "/verification"
	verificationSessionEndpoint = "/session"
	diditWebhookEndpoint        = "/webhooks/didit"
	maxDiditWebhookBodyBytes    = int64(1 << 20)
)

type verificationSessionRequest struct {
	UserType string `json:"type" binding:"required"`
}

type verificationService interface {
	CreateOrResumeSession(
		context.Context,
		string,
		string,
	) (*service.VerificationSessionResponse, error)
	ReceiveDiditWebhook(
		[]byte,
		service.DiditWebhookHeaders,
		time.Time,
	) (service.DiditWebhookReceipt, error)
}

type verificationHandler struct {
	service verificationService
}

func NewVerificationHandler(
	groupHandler *groupHandler,
	verificationService *service.VerificationService,
) {
	h := verificationHandler{service: verificationService}
	auth := middleware.Authorization(config.Config.Jwt.Secret)
	groupHandler.AddEndpointGroupHandler(EndpointGroupHandler{
		Root:       baseVerificationEndpoint,
		Middleware: []gin.HandlerFunc{auth},
		EndpointHandlers: []EndpointHandler{
			{Method: http.MethodPost, Path: verificationSessionEndpoint, HandlerFunc: h.createOrResumeSession},
		},
	})
	groupHandler.AddEndpointGroupHandler(EndpointGroupHandler{
		Root:       baseVerificationEndpoint,
		Middleware: []gin.HandlerFunc{},
		EndpointHandlers: []EndpointHandler{
			{Method: http.MethodPost, Path: diditWebhookEndpoint, HandlerFunc: h.processDiditWebhook},
		},
	})
}

func (h verificationHandler) createOrResumeSession(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}
	address, err := middleware.AddressFromBearer(c)
	if err != nil {
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}
	var request verificationSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "invalid verification session request")
		return
	}
	response, err := h.service.CreateOrResumeSession(c.Request.Context(), address, request.UserType)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrVerificationReconciliationPending) {
			status = http.StatusConflict
		}
		model.JsonResponse(c, status, nil, nodeAddress, err.Error())
		return
	}
	if err := response.Validate(); err != nil {
		model.JsonResponse(c, http.StatusBadGateway, nil, nodeAddress, err.Error())
		return
	}
	model.JsonResponse(c, http.StatusOK, response, nodeAddress, "")
}

func (h verificationHandler) processDiditWebhook(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDiditWebhookBodyBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		status := http.StatusBadRequest
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			status = http.StatusRequestEntityTooLarge
		}
		c.JSON(status, gin.H{"accepted": false})
		return
	}
	receipt, err := h.service.ReceiveDiditWebhook(body, service.DiditWebhookHeaders{
		Timestamp:   c.GetHeader("X-Timestamp"),
		SignatureV2: c.GetHeader("X-Signature-V2"),
		Signature:   c.GetHeader("X-Signature"),
		Simple:      c.GetHeader("X-Signature-Simple"),
		TestWebhook: c.GetHeader("X-Didit-Test-Webhook") == "true",
	}, time.Now().UTC())
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrDiditInvalidSignature) ||
			errors.Is(err, service.ErrDiditStaleWebhook) {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"accepted": false})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"accepted":  true,
		"duplicate": receipt.Duplicate,
		"testOnly":  receipt.TestOnly,
	})
}
