package handlers

import (
	"errors"
	"math/big"
	"net/http"
	"sync"

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
			return
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
			return
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
			return
		case "initailMinted":
			c.JSON(http.StatusOK, 0)
			return
		case "maxSupply":
			c.JSON(http.StatusOK, 161803398)
			return
		}
	}

	latestBlock, err := service.GetLastBlockNumber(client)
	if err != nil {
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}
	var trimmedSupply, trimmedMinted, trimmedBurned *big.Int
	var wg sync.WaitGroup
	var syncErr error
	wg.Add(3)
	go func() {
		defer wg.Done()
		totalSupply, err := service.GetTotalSupply(client, latestBlock, contractAddress)
		if err != nil {
			syncErr = errors.New("error while retrieving total supply: " + err.Error())
			return
		}

		trimmedSupply = big.NewInt(0).Div(totalSupply, oneToken)
	}()
	go func() {
		defer wg.Done()
		totalMinted, err := service.GetTotalMintedAmount(client, latestBlock, contractAddress)
		if err != nil {
			syncErr = errors.New("error while retrieving minted supply: " + err.Error())
			return
		}

		trimmedMinted = big.NewInt(0).Div(totalMinted, oneToken)
	}()
	go func() {
		defer wg.Done()
		totalBurned, err := service.GetTotalBurnedAmount(client, latestBlock, contractAddress)
		if err != nil {
			syncErr = errors.New("error while retrieving burned supply: " + err.Error())
			return
		}

		trimmedBurned = big.NewInt(0).Div(totalBurned, oneToken)
	}()
	wg.Wait()

	if syncErr != nil {
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, syncErr.Error())
		return
	}

	response := tokenSupplyResponse{
		InitilaMinted:     0,
		MaxSupply:         161803398,
		TotalSupply:       trimmedSupply.Int64(),
		CirculatingSupply: trimmedSupply.Int64(),
		Burned:            trimmedBurned.Int64(),
		Minted:            trimmedMinted.Int64(),
	}

	c.JSON(http.StatusOK, response)
}
