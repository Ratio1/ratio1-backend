package middleware

import (
	"errors"
	"net/http"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/gin-gonic/gin"
)

// RequireApprovedKyc admits a request only when the bearer's wallet passes
// service.RequireApprovedKyc; unverified wallets get 403. It must run after
// Authorization. Testnet and devnet (Api.DevTesting) skip KYC, like the license
// endpoints.
func RequireApprovedKyc() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.Config.Api.DevTesting {
			c.Next()
			return
		}

		nodeAddress, err := service.GetAddress()
		if err != nil {
			model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
			c.Abort()
			return
		}

		address, err := AddressFromBearer(c)
		if err != nil {
			model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, err.Error())
			c.Abort()
			return
		}

		if err := service.RequireApprovedKyc(address); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, service.ErrorKycNotApproved) {
				status = http.StatusForbidden
			}
			model.JsonResponse(c, status, nil, nodeAddress, err.Error())
			c.Abort()
			return
		}
		c.Next()
	}
}
