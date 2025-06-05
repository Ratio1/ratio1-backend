package handlers

import (
	"net/http"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
)

const (
	baseAccountEndpoint   = "/accounts"
	getAccountEndpoint    = "/account"
	registerEmailEndpoint = "/email/register"
	confirmEmailEndpoint  = "/email/confirm"
	subscribeEndpoint     = "/subscribe"
	unsubscribeEndpoint   = "/unsubscribe"
	blacklistEndpoint     = "/blacklist"
	addSellerCodeEndpoint = "/add-seller-code"
)

type registerEmailRequest struct {
	Email          string `json:"email"`
	ReceiveUpdates bool   `json:"receiveUpdates"`
}

type blaclistUserRequest struct {
	Address string `json:"address"`
	Reasons string `json:"reasons"`
}

type accountHandler struct{}

func NewAccountHandler(groupHandler *groupHandler) {
	h := accountHandler{}

	publicEndpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: confirmEmailEndpoint, HandlerFunc: h.confirmEmail},
	}

	publicEndpointsGroupHandler := EndpointGroupHandler{
		Root:             baseAccountEndpoint,
		Middleware:       []gin.HandlerFunc{},
		EndpointHandlers: publicEndpoints,
	}
	groupHandler.AddEndpointGroupHandler(publicEndpointsGroupHandler)

	authEndpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: getAccountEndpoint, HandlerFunc: h.getOrCreateAccount},
		{Method: http.MethodPost, Path: registerEmailEndpoint, HandlerFunc: h.registerEmail},
		{Method: http.MethodGet, Path: subscribeEndpoint, HandlerFunc: h.subscribe},
		{Method: http.MethodGet, Path: unsubscribeEndpoint, HandlerFunc: h.unsubscribe},
		{Method: http.MethodPost, Path: blacklistEndpoint, HandlerFunc: h.blackListAccount},
		{Method: http.MethodPost, Path: addSellerCodeEndpoint, HandlerFunc: h.addSellerCode},
	}

	auth := middleware.Authorization(config.Config.Jwt.Secret)
	authEndpointGroupHandler := EndpointGroupHandler{
		Root:             baseAccountEndpoint,
		Middleware:       []gin.HandlerFunc{auth},
		EndpointHandlers: authEndpoints,
	}
	groupHandler.AddEndpointGroupHandler(authEndpointGroupHandler)
}

func (h *accountHandler) getOrCreateAccount(c *gin.Context) {
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

	account, err := service.GetOrCreateAccount(address)
	if err != nil {
		log.Error("error while retrieving account information: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
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

	var kyc *model.Kyc
	if account.Email != nil {
		kyc, _, err = storage.GetKycByEmail(*account.Email)
		if err != nil {
			log.Error("error while retrieving kyc information from storage: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
	}

	accountDto, err := service.NewAccountDto(account, kyc)
	if err != nil {
		log.Error("error while creating account dto: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, accountDto, nodeAddress, "")
}

func (h *accountHandler) registerEmail(c *gin.Context) {
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

	var req registerEmailRequest
	err = c.Bind(&req)
	if err != nil {
		log.Error("error while binding request: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	account, err := service.RegisterEmail(address, req.Email, req.ReceiveUpdates)
	if err != nil {
		log.Error("error while register email: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	var kyc *model.Kyc
	if account.Email != nil {
		kyc, _, err = storage.GetKycByEmail(*account.Email)
		if err != nil {
			log.Error("error while retrieving kyc information from storage: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
	}

	accountDto, err := service.NewAccountDto(account, kyc)
	if err != nil {
		log.Error("error while creating account dto: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, accountDto, nodeAddress, "")
}

func (h *accountHandler) confirmEmail(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	token, ok := c.GetQuery("token")
	if !ok {
		log.Error("error while retrieving token from params")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "empty or invalid token query")
		return
	}

	account, err := service.ConfirmEmail(token)
	if err != nil {
		log.Error("error while confirming email: " + err.Error())
		model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, err.Error())
		return
	}

	kyc, _, err := storage.GetKycByEmail(*account.Email)
	if err != nil {
		log.Error("error while retrieving kyc information from storage: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	accountDto, err := service.NewAccountDto(account, kyc)
	if err != nil {
		log.Error("error while creating account dto: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, accountDto, nodeAddress, "")
}

func (h *accountHandler) subscribe(c *gin.Context) {
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

	err = service.SubscribeEmail(kyc)
	if err != nil {
		log.Error("error while subribing user: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	accountDto, err := service.NewAccountDto(account, kyc)
	if err != nil {
		log.Error("error while creating account dto: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, accountDto, nodeAddress, "")
}

func (h *accountHandler) unsubscribe(c *gin.Context) {
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

	err = service.UnsubscribeEmail(kyc)
	if err != nil {
		log.Error("error while unsubscribing user: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	accountDto, err := service.NewAccountDto(account, kyc)
	if err != nil {
		log.Error("error while creating account dto: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, accountDto, nodeAddress, "")
}

func (h *accountHandler) blackListAccount(c *gin.Context) {
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

	isAdmin := false
	for _, admin := range config.Config.AdminAddresses {
		if address == admin {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		log.Error("address not authorized, user is not admin!")
		model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "not authorized")
		return
	}

	var blockAccount blaclistUserRequest
	err = c.Bind(&blockAccount)
	if err != nil {
		log.Error("error while binding request: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	account, err := service.GetOrCreateAccount(blockAccount.Address)
	if err != nil {
		log.Error("error while retrieving account information: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	account.IsBlacklisted = true
	account.BlacklistedReason = &blockAccount.Reasons

	err = service.SendBlacklistedEmail(*account.Email)
	if err != nil {
		log.Error("error while sending blacklisted email: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	kyc, _, err := storage.GetKycByEmail(*account.Email)
	if err != nil {
		log.Error("error while retrieving kyc information from storage: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	accountDto, err := service.NewAccountDto(account, kyc)
	if err != nil {
		log.Error("error while creating account dto: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, accountDto, nodeAddress, "")
}

func (h *accountHandler) addSellerCode(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	referralCode, ok := c.GetQuery("sellerCode")
	if !ok || referralCode == "" {
		log.Error("error while retrieving referral code from params")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "empty or invalid referral code query")
		return
	}

	address, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	account, err := service.GetOrCreateAccount(address)
	if err != nil {
		log.Error("error while retrieving account information: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
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

	exist, err := storage.SellerCodeDoExist(referralCode)
	if err != nil {
		log.Error("error while checking referral code: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	if !exist {
		log.Error("referral code does not exist")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "referral code does not exist")
		return
	}

	sellerCode, err := storage.GetSellerCodeByAddress(address)
	if err != nil {
		log.Error("error while retrieving seller code: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving seller code: "+err.Error())
		return
	} else if sellerCode != nil && *sellerCode == referralCode {
		log.Error("user is using his own seller code")
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "user is using his own seller code")
		return
	}

	account.UsedSellerCode = &referralCode
	err = storage.UpdateAccount(account)
	if err != nil {
		log.Error("error while updating account: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	var kyc *model.Kyc
	if account.Email != nil {
		kyc, _, err = storage.GetKycByEmail(*account.Email)
		if err != nil {
			log.Error("error while retrieving kyc information from storage: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
	}

	accountDto, err := service.NewAccountDto(account, kyc)
	if err != nil {
		log.Error("error while creating account dto: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, accountDto, nodeAddress, "")
}
