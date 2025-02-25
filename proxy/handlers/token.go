package handlers

import (
	"math/big"
	"net/http"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
)

const (
	baseTokenEndpoint = "/token"
	getSupplyEndpoint = "/supply"
)

type tokenSupplyResponse struct {
	Supply            string `json:"supply"`
	CirculatingSupply string `json:"circulatingSupply"`
	Minted            string `json:"minted"`
	Burned            string `json:"burned"`
	InitilaMinted     string `json:"initialMinted"`
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

	contractAddress := common.HexToAddress(config.Config.R1ContractAddress)
	supply, err := service.GetTokenSupplyInfo(contractAddress)
	if err != nil {
		log.Error("error while retrieving token informations: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	oneToken := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)
	trimmedSuply := big.NewInt(0).Div(supply.Supply, oneToken)
	trimmedMinted := big.NewInt(0).Div(supply.Minted, oneToken)
	trimmedBurned := big.NewInt(0).Div(supply.Burned, oneToken)

	response := tokenSupplyResponse{
		InitilaMinted:     "0",
		Supply:            trimmedSuply.String(),
		CirculatingSupply: trimmedSuply.String(),
		Burned:            trimmedBurned.String(),
		Minted:            trimmedMinted.String(),
	}

	c.JSON(http.StatusOK, response)
}
