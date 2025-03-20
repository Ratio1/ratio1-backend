package handlers

import (
	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/gin-gonic/gin"
)

var log = logger.GetOrCreate("handlers")

type EndpointGroupHandler struct {
	Root             string
	Middleware       []gin.HandlerFunc
	EndpointHandlers []EndpointHandler
}

type EndpointHandler struct {
	Path        string
	Method      string
	HandlerFunc gin.HandlerFunc
}

type groupHandler struct {
	endpointHandlersMap map[string][]EndpointGroupHandler
}

func NewGroupHandler() *groupHandler {
	return &groupHandler{
		endpointHandlersMap: make(map[string][]EndpointGroupHandler),
	}
}

func (g *groupHandler) RegisterEndpoints(r *gin.Engine) {
	for groupRoot, handlersGroups := range g.endpointHandlersMap {
		for _, handlersGroup := range handlersGroups {
			routerGroup := r.Group(groupRoot).Use(handlersGroup.Middleware...)
			{
				for _, h := range handlersGroup.EndpointHandlers {
					routerGroup.Handle(h.Method, h.Path, h.HandlerFunc)
				}
			}
		}
	}
}

func (g *groupHandler) AddEndpointGroupHandler(endpointHandler EndpointGroupHandler) {
	g.endpointHandlersMap[endpointHandler.Root] = append(g.endpointHandlersMap[endpointHandler.Root], endpointHandler)
}
