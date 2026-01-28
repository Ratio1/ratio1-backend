package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/crypto"
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
	logoutAuthEndpoint  = "/logout"
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
	Expiration int64 `json:"expiration"`
}

type authHandler struct{}

func NewAuthHandler(groupHandler *groupHandler) {
	h := authHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: accessAuthEndpoint, HandlerFunc: h.createAccessToken},
		{Method: http.MethodPost, Path: refreshAuthEndpoint, HandlerFunc: h.refreshAccessToken},
		{Method: http.MethodPost, Path: logoutAuthEndpoint, HandlerFunc: h.logout},
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
		safeErr := crypto.VerifySafeSignature(message.GetAddress().String(), req.Message, req.Signature)
		if safeErr != nil {
			log.Error("error while verifying signature: " + safeErr.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, safeErr.Error())
			return
		}
	}

	//create bearer token
	jwt, refresh, err := service.MakeJwtAndRefresh(message.GetAddress().String())
	if err != nil {
		log.Error("error while jwt generation: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	setAuthCookies(c, jwt, refresh)
	model.JsonResponse(c, http.StatusOK, tokenPayload{
		Expiration: time.Now().Unix() + int64(config.Config.Jwt.ExpiryMins*60),
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
	err = c.ShouldBindJSON(&req)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Error("error while binding request: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	refreshToken := strings.TrimSpace(req.Token)
	if refreshToken == "" {
		cookieToken, ok := readAuthCookie(c, config.Config.Jwt.RefreshCookieName)
		if !ok {
			log.Error("missing refresh token")
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "Missing refresh token")
			return
		}
		refreshToken = cookieToken
	}

	bearer := c.Request.Header.Get("Authorization")
	accessToken := ""
	if bearer == "" {
		cookieToken, ok := readAuthCookie(c, config.Config.Jwt.AccessCookieName)
		if !ok {
			log.Error("missing access token")
			model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "Missing access token")
			return
		}
		accessToken = cookieToken
	} else {
		ok, token := middleware.ParseBearer(bearer)
		if !ok {
			log.Error("cannot parse bearer")
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "Can't parse bearer")
			return
		}
		accessToken = token
	}

	jwt, refresh, err := service.RefreshToken(accessToken, refreshToken)
	if err != nil {
		log.Error("error while refreshing token: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	setAuthCookies(c, jwt, refresh)
	model.JsonResponse(c, http.StatusOK, tokenPayload{
		Expiration: time.Now().Unix() + int64(config.Config.Jwt.ExpiryMins*60),
	}, nodeAddress, "")
}

func (h *authHandler) logout(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	clearAuthCookies(c)
	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
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

func setAuthCookies(c *gin.Context, accessToken, refreshToken string) {
	maxAge := config.Config.Jwt.ExpiryMins * 60
	setAuthCookie(c, config.Config.Jwt.AccessCookieName, accessToken, maxAge)
	setAuthCookie(c, config.Config.Jwt.RefreshCookieName, refreshToken, maxAge)
}

func clearAuthCookies(c *gin.Context) {
	setAuthCookie(c, config.Config.Jwt.AccessCookieName, "", -1)
	setAuthCookie(c, config.Config.Jwt.RefreshCookieName, "", -1)
}

func readAuthCookie(c *gin.Context, name string) (string, bool) {
	if name == "" {
		return "", false
	}
	value, err := c.Cookie(name)
	if err != nil || value == "" {
		return "", false
	}
	return value, true
}

func setAuthCookie(c *gin.Context, name, value string, maxAge int) {
	if name == "" {
		return
	}
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   config.Config.Jwt.CookieDomain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   config.Config.Jwt.CookieSecure,
		SameSite: cookieSameSiteMode(),
	}
	if maxAge < 0 {
		cookie.Expires = time.Unix(0, 0)
	} else if maxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(maxAge) * time.Second)
	}
	http.SetCookie(c.Writer, cookie)
}

func cookieSameSiteMode() http.SameSite {
	switch strings.ToLower(strings.TrimSpace(config.Config.Jwt.CookieSameSite)) {
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	case "lax":
		return http.SameSiteLaxMode
	case "default":
		return http.SameSiteDefaultMode
	default:
		return http.SameSiteLaxMode
	}
}
