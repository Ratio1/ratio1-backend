package proxy

import (
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
	router *gin.Engine
}

func NewWebServer() (*WebServer, error) {
	router := gin.Default()
	corsCfg := cors.DefaultConfig()
	corsCfg.AllowHeaders = corsHeaders
	corsCfg.AllowAllOrigins = false
	corsCfg.AllowOrigins = buildCorsOrigins()
	corsCfg.AllowCredentials = true
	router.Use(cors.New(corsCfg))
	router.Static("../public", "./public")

	groupHandler := handlers.NewGroupHandler()

	service.NewAuthService()

	handlers.NewAuthHandler(groupHandler)
	handlers.NewLaunchpadHandler(groupHandler)
	handlers.NewAccountHandler(groupHandler)
	handlers.NewSumsubHandler(groupHandler)
	handlers.NewTokenHandler(groupHandler)
	handlers.NewSellerHandler(groupHandler)
	handlers.NewAdminHandler(groupHandler)
	handlers.NewInvoiceDraftHandler(groupHandler)
	handlers.NewBurnReportHandler(groupHandler)
	handlers.NewBrandingHandler(groupHandler)

	groupHandler.RegisterEndpoints(router)

	return &WebServer{
		router: router,
	}, nil
}

func buildCorsOrigins() []string {
	origins := make([]string, 0, len(config.Config.AcceptedDomains.Inner)*2)
	seen := make(map[string]struct{})
	for _, domain := range config.Config.AcceptedDomains.Inner {
		trimmed := strings.TrimSpace(domain.Domain)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			origins = addCorsOrigin(origins, seen, trimmed)
			continue
		}
		if strings.Contains(trimmed, "localhost") || strings.HasPrefix(trimmed, "127.0.0.1") {
			origins = addCorsOrigin(origins, seen, "http://"+trimmed)
			origins = addCorsOrigin(origins, seen, "https://"+trimmed)
			continue
		}
		origins = addCorsOrigin(origins, seen, "https://"+trimmed)
	}
	return origins
}

func addCorsOrigin(origins []string, seen map[string]struct{}, origin string) []string {
	if _, ok := seen[origin]; ok {
		return origins
	}
	seen[origin] = struct{}{}
	return append(origins, origin)
}

func (w *WebServer) Run() *http.Server {
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
