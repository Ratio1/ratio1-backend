package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	baseSumsubEndpoint        = "/sumsub"
	kycInitEndpoint           = "/init/Kyc"
	hookEndpoint              = "/hook"
	maxSumsubWebhookBodyBytes = int64(1 << 20)
)

type initSessionRequest struct {
	UserType string `json:"type"`
}

type sumsubHandler struct{}

func NewSumsubHandler(groupHandler *groupHandler) {
	h := sumsubHandler{}

	auth := middleware.Authorization(config.Config.Jwt.Secret)
	authEndpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: kycInitEndpoint, HandlerFunc: h.initSession},
	}
	authEndpointsGroup := EndpointGroupHandler{
		Root:             baseSumsubEndpoint,
		Middleware:       []gin.HandlerFunc{auth},
		EndpointHandlers: authEndpoints,
	}
	groupHandler.AddEndpointGroupHandler(authEndpointsGroup)

	publicEndpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: hookEndpoint, HandlerFunc: h.processEvents},
	}

	publicEndpointsGroup := EndpointGroupHandler{
		Root:             baseSumsubEndpoint,
		Middleware:       []gin.HandlerFunc{},
		EndpointHandlers: publicEndpoints,
	}

	groupHandler.AddEndpointGroupHandler(publicEndpointsGroup)

}

func (h *sumsubHandler) initSession(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}
	if config.Config.Verification.Provider != model.VerificationProviderSumsub {
		model.JsonResponse(c, http.StatusGone, nil, nodeAddress, "Sumsub onboarding is inactive")
		return
	}

	address, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	var req initSessionRequest
	err = c.Bind(&req)
	if err != nil {
		log.Error("error while binding request: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	account, err := service.GetOrCreateAccount(address)
	if err != nil {
		log.Error("error while retrieving account information: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	if !account.EmailConfirmed {
		log.Error("email is not confirmed")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "email is not confirmed")
		return
	}

	if account.IsBlacklisted {
		if account.BlacklistedReason != nil {
			log.Error("account: " + address + " is blacklisted with reason: " + *account.BlacklistedReason)
			model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "account is blacklisted with reason:"+*account.BlacklistedReason)
			return
		} else {
			log.Error("account: " + address + " is blacklisted!")
			model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "account is blacklisted")
			return
		}
	}

	kyc, found, err := storage.GetKycByEmail(*account.Email)
	if err != nil {
		log.Error("error while retrieving kyc information from storage: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	} else if !found {
		log.Error("kyc not found in storage")
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "user email not found")
		return
	}

	if kyc.KycStatus == model.StatusFinalRejected {
		log.Error("user is final rejected, cannot retry")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "user is final rejected, cannot retry")
		return
	}
	if kyc.VerificationProvider != "" &&
		kyc.VerificationProvider != model.VerificationProviderSumsub {
		model.JsonResponse(
			c,
			http.StatusConflict,
			nil,
			nodeAddress,
			"verification is owned by another provider and requires an explicit rollback",
		)
		return
	}

	//User never init kyc
	if kyc.ApplicantType == "" {
		if req.UserType == model.BusinessCustomer {
			kyc.ApplicantType = model.BusinessCustomer
		} else if req.UserType == model.IndividualCustomer {
			kyc.ApplicantType = model.IndividualCustomer
		} else {
			log.Error("wrong request parametere sent")
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "wrong request parameter sent")
			return
		}
	}

	var level string
	if kyc.ApplicantType == model.BusinessCustomer {
		level = config.Config.Sumsub.BusinessLevelName
	} else if kyc.ApplicantType == model.IndividualCustomer {
		level = config.Config.Sumsub.CustomerLevelName
	} else {
		log.Error("invalid applicant type: " + kyc.ApplicantType)
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "invalid applicant type: "+kyc.ApplicantType)
		return
	}

	token, err := service.InitNewSession(kyc.Uuid.String(), level)
	if err != nil {
		log.Error("error while starting new kyc session: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	kyc.VerificationProvider = model.VerificationProviderSumsub
	err = storage.CreateOrUpdateKyc(kyc)
	if err != nil {
		log.Error("error while saving kyc information in storage: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, token, nodeAddress, "")
}

func (h *sumsubHandler) processEvents(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSumsubWebhookBodyBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		status := http.StatusBadRequest
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			status = http.StatusRequestEntityTooLarge
		}
		model.JsonResponse(c, status, nil, "", "invalid Sumsub webhook body")
		return
	}

	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}
	if config.Config.Verification.Provider != model.VerificationProviderSumsub &&
		!config.Config.Verification.LegacySumsubWebhooksEnabled {
		model.JsonResponse(c, http.StatusGone, nil, nodeAddress, "Sumsub monitoring is inactive")
		return
	}

	err = h.validateSecret(c, body)
	if err != nil {
		log.Error("error while validating secret: " + err.Error())
		model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, err.Error())
		return
	}

	var kycEvent model.SumsubEvent
	err = json.Unmarshal(body, &kycEvent)
	if err != nil {
		log.Error("error while binding request: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	if checkIfBeneficiaryUUID(kycEvent.ExternalUserID) {
		model.JsonResponse(c, http.StatusOK, "External user id found", nodeAddress, "")
		return
	}

	uuid, err := uuid.Parse(kycEvent.ExternalUserID)
	if err != nil {
		log.Error("error while parsing user uuid: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	kyc, found, err := storage.GetKycByUuid(uuid)
	if err != nil {
		log.Error("error while retrieving kyc information from storage: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	} else if !found {
		log.Error("kyc not found in storage")
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "user email not found")
		return
	}

	if config.Config.Verification.Provider == model.VerificationProviderSumsub {
		fullKyc, shouldProcess := service.PrepareSumsubKycForFullProcessing(*kyc)
		if !shouldProcess {
			model.JsonResponse(c, http.StatusOK, "", nodeAddress, "")
			return
		}
		if fullKyc.KycStatus == model.StatusFinalRejected &&
			kycEvent.Type != model.ApplicantReset {
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "user is final rejected, cannot retry")
			return
		}
		user, accountFound, accountErr := storage.GetAccountByEmail(fullKyc.Email)
		if accountErr != nil {
			log.Error("error while retrieving account information from storage: " + accountErr.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, accountErr.Error())
			return
		}
		if !accountFound {
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "user email not found")
			return
		}
		if err = service.ProcessKycEvent(kycEvent, fullKyc, user.Address); err != nil {
			log.Error("error while processing Sumsub event: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
		model.JsonResponse(c, http.StatusOK, "", nodeAddress, "")
		return
	}

	if kyc.KycStatus == model.StatusFinalRejected {
		model.JsonResponse(c, http.StatusOK, "", nodeAddress, "")
		return
	}
	if kyc.VerificationProvider != model.VerificationProviderSumsub ||
		(kyc.KycStatus != model.StatusApproved && kyc.KycStatus != model.StatusOnHold) ||
		kyc.ApplicantId == "" ||
		kyc.ApplicantId != kycEvent.ApplicantID {
		model.JsonResponse(c, http.StatusOK, "", nodeAddress, "")
		return
	}

	eventId := strings.TrimSpace(kycEvent.CorrelationID)
	if eventId == "" {
		digest := sha256.Sum256(body)
		eventId = "payload:" + hex.EncodeToString(digest[:])
	}
	payloadDigest := sha256.Sum256(body)
	environment := model.VerificationEnvironmentProduction
	if kycEvent.SandboxMode {
		environment = model.VerificationEnvironmentSandbox
	}
	occurredAt, err := service.ParseSumsubMonitoringOccurredAt(kycEvent.CreatedAtMs)
	if err != nil {
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}
	created, err := storage.CreateVerificationWebhookEvent(&model.VerificationWebhookEvent{
		Provider:          model.VerificationProviderSumsub,
		Environment:       environment,
		EventId:           eventId,
		EventType:         kycEvent.Type,
		ProviderSessionId: kycEvent.ApplicantID,
		VendorData:        kycEvent.ExternalUserID,
		ProviderStatus:    kycEvent.ReviewResult.ReviewAnswer,
		StatusReason:      kycEvent.ReviewResult.ReviewRejectType,
		OccurredAt:        &occurredAt,
		ReceivedAt:        time.Now().UTC(),
		PayloadSha256:     hex.EncodeToString(payloadDigest[:]),
		ProcessingStatus:  model.VerificationEventReceived,
	})
	if err != nil {
		log.Error("error while persisting Sumsub monitoring event: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}
	if !created {
		storedEvent, found, readErr := storage.GetVerificationWebhookEvent(
			model.VerificationProviderSumsub,
			environment,
			eventId,
		)
		if readErr != nil {
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, readErr.Error())
			return
		}
		if found && storedEvent.ProcessingStatus == model.VerificationEventProcessed {
			model.JsonResponse(c, http.StatusOK, "", nodeAddress, "")
			return
		}
	}
	model.JsonResponse(c, http.StatusOK, "", nodeAddress, "")
}

func (h *sumsubHandler) validateSecret(c *gin.Context, body []byte) error {
	signatureType := c.GetHeader("X-Payload-Digest-Alg")
	if signatureType != "HMAC_SHA256_HEX" {
		return errors.New("invalid algorythm provided")
	}
	digest := c.GetHeader("x-payload-digest")
	if digest == "" {
		return errors.New("empty digest")
	}

	calculatedDigest := _calculateHMAC(body, config.Config.Sumsub.SumsubJwtSecretKey, sha256.New)

	if !hmac.Equal([]byte(digest), []byte(calculatedDigest)) {
		return errors.New("invalid signature")
	}
	return nil
}

func _calculateHMAC(message []byte, secret string, hashFunc func() hash.Hash) string {
	h := hmac.New(hashFunc, []byte(secret))
	h.Write(message)
	return hex.EncodeToString(h.Sum(nil))
}

func checkIfBeneficiaryUUID(uuidAsString string) bool {
	if strings := strings.Split(uuidAsString, "-"); strings[0] == "beneficiary" {
		return true
	}
	return false
}
