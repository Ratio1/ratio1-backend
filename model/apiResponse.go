package model

import (
	"github.com/gin-gonic/gin"
)

type ApiResponse struct {
	Data        interface{} `json:"data"`
	NodeAddress string      `json:"nodeAddress"`
	Error       string      `json:"error"`
}

func JsonResponse(c *gin.Context, status int, data interface{}, nodeAddress, error string) {
	c.JSON(status, ApiResponse{
		Data:        data,
		NodeAddress: nodeAddress,
		Error:       error,
	})
}
