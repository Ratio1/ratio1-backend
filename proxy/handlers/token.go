package handlers

import (
	"math/big"
	"net/http"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
)

const (
	baseTokenEndpoint = "/token"
	getSupplyEndpoint = "/supply"
)

type tokenSupplyResponse struct {
	CirculatingSupply string `json:"circulatingSupply"`
	TotalSupply       string `json:"totalSupply"`
	MaxSupply         string `json:"maxSupply"`
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
	oneToken := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while dialing client") //ERROR is not returned because it can expose secret key
	}
	defer client.Close()

	token, ok := c.GetQuery("extract")
	if ok && token != "" {
		switch token {
		case "circulatingSupply", "totalSupply":
			latestBlock, err := service.GetLastBlockNumber(client)
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			totalSupply, err := service.GetTotalSupply(client, latestBlock, contractAddress)
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedSuply := big.NewInt(0).Div(totalSupply, oneToken)
			c.JSON(http.StatusOK, trimmedSuply)
		case "minted":
			latestBlock, err := service.GetLastBlockNumber(client)
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			totalMinted, err := service.GetTotalMintedAmount(client, latestBlock, contractAddress)
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedMinted := big.NewInt(0).Div(totalMinted, oneToken)
			c.JSON(http.StatusOK, trimmedMinted)
		case "burned":
			latestBlock, err := service.GetLastBlockNumber(client)
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			totalBurned, err := service.GetTotalBurnedAmount(client, latestBlock, contractAddress)
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedBurned := big.NewInt(0).Div(totalBurned, oneToken)
			c.JSON(http.StatusOK, trimmedBurned)
		case "initailMinted":
			c.JSON(http.StatusOK, 0)
		case "maxSupply":
			c.JSON(http.StatusOK, 161803398)
		default:
			supply, err := service.GetTokenSupplyInfo(contractAddress)
			if err != nil {
				log.Error("error while retrieving token informations: " + err.Error())
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedSuply := big.NewInt(0).Div(supply.Supply, oneToken)
			trimmedMinted := big.NewInt(0).Div(supply.Minted, oneToken)
			trimmedBurned := big.NewInt(0).Div(supply.Burned, oneToken)

			response := tokenSupplyResponse{
				InitilaMinted:     "0",
				MaxSupply:         "161803398",
				TotalSupply:       trimmedSuply.String(),
				CirculatingSupply: trimmedSuply.String(),
				Burned:            trimmedBurned.String(),
				Minted:            trimmedMinted.String(),
			}

			c.JSON(http.StatusOK, response)
		}
	} else {
		supply, err := service.GetTokenSupplyInfo(contractAddress)
		if err != nil {
			log.Error("error while retrieving token informations: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}

		trimmedSuply := big.NewInt(0).Div(supply.Supply, oneToken)
		trimmedMinted := big.NewInt(0).Div(supply.Minted, oneToken)
		trimmedBurned := big.NewInt(0).Div(supply.Burned, oneToken)

		response := tokenSupplyResponse{
			InitilaMinted:     "0",
			MaxSupply:         "161803398",
			TotalSupply:       trimmedSuply.String(),
			CirculatingSupply: trimmedSuply.String(),
			Burned:            trimmedBurned.String(),
			Minted:            trimmedMinted.String(), //TODO add total supply 161803398
		}

		c.JSON(http.StatusOK, response)
	}
}
