package proxy

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/handlers"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var corsHeaders = []string{
	"Origin",
	"Content-Length",
	"Content-Type",
	"Authorization",
}

type WebServer struct {
	router              *gin.Engine
	verificationService *service.VerificationService
	verificationWorker  *service.VerificationWorker
}

func NewWebServer() (*WebServer, error) {
	router := gin.Default()
	corsCfg := cors.DefaultConfig()
	corsCfg.AllowHeaders = corsHeaders
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowCredentials = true
	router.Use(cors.New(corsCfg))
	router.Static("../public", "./public")

	groupHandler := handlers.NewGroupHandler()

	service.NewAuthService()
	verificationService, err := service.NewVerificationService(config.Config)
	if err != nil {
		return nil, err
	}

	handlers.NewAuthHandler(groupHandler)
	handlers.NewHealthHandler(groupHandler)
	handlers.NewLaunchpadHandler(groupHandler)
	handlers.NewAccountHandler(groupHandler)
	handlers.NewSumsubHandler(groupHandler)
	handlers.NewVerificationHandler(groupHandler, verificationService)
	handlers.NewTokenHandler(groupHandler)
	handlers.NewSellerHandler(groupHandler)
	handlers.NewAdminHandler(groupHandler)
	handlers.NewInvoiceDraftHandler(groupHandler)
	handlers.NewBurnReportHandler(groupHandler)
	handlers.NewBrandingHandler(groupHandler)

	groupHandler.RegisterEndpoints(router)

	return &WebServer{
		router:              router,
		verificationService: verificationService,
	}, nil
}

func (w *WebServer) Run() *http.Server {
	w.verificationWorker = w.verificationService.StartWorker(context.Background())
	address := config.Config.Api.Address
	if !strings.Contains(address, ":") {
		panic("bad address")
	}

	// Log the server startup
	logger := handlers.GetLogger()
	logger.Info("Starting API server", "address", address)

	server := &http.Server{
		Addr:           address,
		Handler:        w.router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		logger.Info("API server is running", "address", address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err.Error())
			panic(err)
		}
	}()

	return server
}

func (w *WebServer) Shutdown(ctx context.Context, server *http.Server) error {
	if w.verificationWorker != nil {
		w.verificationWorker.Stop()
	}
	return server.Shutdown(ctx)
}
