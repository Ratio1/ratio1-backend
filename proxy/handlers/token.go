package handlers

import (
	"net/http"
	"sync"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/gin-gonic/gin"
)

const (
	baseTokenEndpoint = "/token"
	getSupplyEndpoint = "/supply"
)

type tokenSupplyResponse struct {
	CirculatingSupply int64 `json:"circulatingSupply"`
	TotalSupply       int64 `json:"totalSupply"`
	MaxSupply         int64 `json:"maxSupply"`
	Minted            int64 `json:"minted"`
	Burned            int64 `json:"burned"`
	InitilaMinted     int64 `json:"initialMinted"`
}

type tokenHandler struct{}

func NewTokenHandler(groupHandler *groupHandler) {
	h := tokenHandler{}

	publicEndpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: getSupplyEndpoint, HandlerFunc: h.getTokenSupply},
	}

	publicEndpointsGroupHandler := EndpointGroupHandler{
		Root:             baseTokenEndpoint,
		Middleware:       []gin.HandlerFunc{},
		EndpointHandlers: publicEndpoints,
	}
	groupHandler.AddEndpointGroupHandler(publicEndpointsGroupHandler)
}

func (h *tokenHandler) getTokenSupply(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	if config.Config.Api.DevTesting {
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "node is not on mainnet")
		return
	}

	if config.Config.R1ContractAddress == "" {
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "R1 address not present")
		return
	}

	token, ok := c.GetQuery("extract")
	if ok && token != "" {
		switch token {
		case "circulatingSupply", "totalSupply":
			totalSupply, err := service.GetTotalSupply()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			c.String(http.StatusOK, "%d", totalSupply)
			return
		case "minted":
			totalMinted, err := service.GetTotalMintedAmount()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			c.String(http.StatusOK, "%d", totalMinted)
			return
		case "burned":
			totalBurned, err := service.GetTotalBurnedAmount()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			c.String(http.StatusOK, "%d", totalBurned)
			return
		case "initailMinted":
			c.String(http.StatusOK, "%d", 0)
			return
		case "maxSupply":
			c.String(http.StatusOK, "%d", 161803398)
			return
		}
	}

	var trimmedSupply, trimmedMinted, trimmedBurned int64
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		trimmedSupply, err = service.GetTotalSupply()
	}()
	go func() {
		defer wg.Done()
		trimmedMinted, err = service.GetTotalMintedAmount()
	}()
	go func() {
		defer wg.Done()
		trimmedBurned, err = service.GetTotalBurnedAmount()
	}()
	wg.Wait()

	if err != nil {
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	response := tokenSupplyResponse{
		InitilaMinted:     0,
		MaxSupply:         161803398,
		TotalSupply:       trimmedSupply,
		CirculatingSupply: trimmedSupply,
		Burned:            trimmedBurned,
		Minted:            trimmedMinted,
	}

	c.JSON(http.StatusOK, response)
}
