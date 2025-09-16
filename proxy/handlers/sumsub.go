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

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	baseSumsubEndpoint = "/sumsub"
	kycInitEndpoint    = "/init/Kyc"
	hookEndpoint       = "/hook"
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

	err = storage.CreateOrUpdateKyc(kyc)
	if err != nil {
		log.Error("error while saving kyc information in storage: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, token, nodeAddress, "")
}

func (h *sumsubHandler) processEvents(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("error while parsing request body: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
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

	if kyc.KycStatus == model.StatusFinalRejected && kycEvent.Type != model.ApplicantReset {
		log.Error("user is final rejected, cannot retry")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "user is final rejected, cannot retry")
		return
	}

	user, found, err := storage.GetAccountByEmail(kyc.Email)
	if err != nil {
		log.Error("error while retrieving account information from storage: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	} else if !found {
		log.Error("account not found in storage")
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "user email not found")
		return
	}

	err = service.ProcessKycEvent(kycEvent, *kyc, user.Address)
	if err != nil {
		log.Error("error whil eprocessing event: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
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
