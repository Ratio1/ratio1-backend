package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
)

const baseHealthEndpoint = "/health"

func NewHealthHandler(groupHandler *groupHandler) {
	groupHandler.AddEndpointGroupHandler(EndpointGroupHandler{
		Root:       baseHealthEndpoint,
		Middleware: []gin.HandlerFunc{},
		EndpointHandlers: []EndpointHandler{
			{Method: http.MethodGet, Path: "/live", HandlerFunc: healthLive},
			{Method: http.MethodGet, Path: "/ready", HandlerFunc: healthReady},
		},
	})
}

func healthLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "live"})
}

func healthReady(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := storage.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":               "ready",
		"verificationProvider": config.Config.Verification.Provider,
	})
}
