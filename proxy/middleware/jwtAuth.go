package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/crypto"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/gin-gonic/gin"
)

const (
	noBearerPresent = "No authorization bearer provided"
	incorrectBearer = "Incorrect bearer provided"
	invalidJwtToken = "Invalid or expired token"

	bearerSplitOn = "Bearer "
	authHeaderKey = "Authorization"

	AddressKey = "address"
)

var returnUnauthorized = func(c *gin.Context, errMessage string) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, errMessage)
}

func AddressFromBearer(c *gin.Context) (string, error) {
	address, ok := c.Get(AddressKey)
	if !ok {
		return "", errors.New("failed to get address claim from bearer")
	}
	addressStr, ok := address.(string)
	if !ok {
		return "", errors.New("got invalid address claim from bearer")
	}
	return addressStr, nil
}

func Authorization(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := c.Request.Header.Get(authHeaderKey)
		token := ""
		if bearer == "" {
			cookieToken, err := c.Cookie(config.Config.Jwt.AccessCookieName)
			if err != nil || cookieToken == "" {
				returnUnauthorized(c, noBearerPresent)
				c.Abort()
				return
			}
			token = cookieToken
		} else {
			ok, bearerToken := ParseBearer(bearer)
			if !ok {
				returnUnauthorized(c, incorrectBearer)
				c.Abort()
				return
			}
			token = bearerToken
		}

		claims, err := crypto.ValidateJwt(token, secret)
		if err != nil {
			returnUnauthorized(c, invalidJwtToken)
			c.Abort()
			return
		}

		c.Set(AddressKey, claims.Address)
		c.Next()
	}
}

func ParseBearer(bearer string) (bool, string) {
	splitBearer := strings.Split(bearer, bearerSplitOn)

	if len(splitBearer) != 2 {
		return false, ""
	}

	return true, strings.TrimSpace(splitBearer[1])
}
