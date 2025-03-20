package handlers

import (
	"net/http"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/gin-gonic/gin"
	"github.com/spruceid/siwe-go"
)

const (
	baseAuthEndpoint    = "/auth"
	accessAuthEndpoint  = "/access"
	refreshAuthEndpoint = "/refresh"
	nodeDataEndpoint    = "/nodeData"
)

type createTokenRequest struct {
	Signature string `json:"signature"`
	Message   string `json:"message"`
}

type refreshTokenRequest struct {
	Token string `json:"refreshToken"`
}

type tokenPayload struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Expiration   int64  `json:"expiration"`
}

type responsePayload struct {
	Version    string `json:"version"`
	NodeAddres string `json:"nodeAddress"`
}

type authHandler struct{}

func NewAuthHandler(groupHandler *groupHandler) {
	h := authHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: accessAuthEndpoint, HandlerFunc: h.createAccessToken},
		{Method: http.MethodPost, Path: refreshAuthEndpoint, HandlerFunc: h.refreshAccessToken},
		{Method: http.MethodGet, Path: nodeDataEndpoint, HandlerFunc: h.getNodeData},
	}

	endpointGroupHandler := EndpointGroupHandler{
		Root:             baseAuthEndpoint,
		Middleware:       []gin.HandlerFunc{},
		EndpointHandlers: endpoints,
	}

	groupHandler.AddEndpointGroupHandler(endpointGroupHandler)
}

func (h *authHandler) createAccessToken(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	//check that the message received is correct
	req := createTokenRequest{}
	err = c.Bind(&req)
	if err != nil {
		log.Error("error while binding request: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	//parse the message received
	message, err := siwe.ParseMessage(req.Message)
	if err != nil {
		log.Error("error while parsing siwe message: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	//verify message
	_, err = message.ValidNow()
	if err != nil {
		log.Error("error while validating message: " + err.Error())
		model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, err.Error())
		return
	}

	if message.GetChainID() != config.Config.ChainID {
		log.Error("wrong chian id retrieved from message")
		model.JsonResponse(c, http.StatusBadRequest, message.GetChainID(), nodeAddress, "wrong chain id, expected 84532")
		return
	}

	isCorrect := false
	for _, domain := range config.Config.AcceptedDomains.Inner {
		if message.GetDomain() == domain.Domain {
			isCorrect = true
		}
	}

	if !isCorrect {
		log.Error("the domain in the message is not whitelisted")
		model.JsonResponse(c, http.StatusBadRequest, message.GetDomain(), nodeAddress, "the domain in the message is not accepted")
		return
	}

	_, err = message.VerifyEIP191(req.Signature)
	if err != nil {
		log.Error("error while verifying signature: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	//create bearer token
	jwt, refresh, err := service.MakeJwtAndRefresh(message.GetAddress().String())
	if err != nil {
		log.Error("error while jwt generation: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, tokenPayload{
		AccessToken:  jwt,
		RefreshToken: refresh,
		Expiration:   time.Now().Unix() + int64(config.Config.Jwt.ExpiryMins*60),
	}, nodeAddress, "")
}

func (h *authHandler) refreshAccessToken(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	req := refreshTokenRequest{}
	err = c.Bind(&req)
	if err != nil {
		log.Error("error while binding request: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	bearer := c.Request.Header.Get("Authorization")
	ok, token := middleware.ParseBearer(bearer)
	if !ok {
		log.Error("cannot parse bearer")
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "Can't parse bearer")
		return
	}

	jwt, refresh, err := service.RefreshToken(token, req.Token)
	if err != nil {
		log.Error("error while refreshing token: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, tokenPayload{
		AccessToken:  jwt,
		RefreshToken: refresh,
		Expiration:   time.Now().Unix() + int64(config.Config.Jwt.ExpiryMins*60),
	}, nodeAddress, "")
}

func (h *authHandler) getNodeData(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}
	model.JsonResponse(c, http.StatusOK, config.BackendVersion, nodeAddress, "")
}
