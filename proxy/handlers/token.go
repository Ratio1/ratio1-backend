package handlers

import (
	"errors"
	"net/http"
	"strings"
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
		case "circulatingSupply":
			totalSupply, err := service.GetTotalSupply()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			teamSupply, err := service.GetTeamWalletsSupply()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			circulatingSupply := totalSupply - teamSupply
			c.String(http.StatusOK, "%d", circulatingSupply)
			return
		case "totalSupply":
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

	var trimmedSupply, trimmedMinted, trimmedBurned, trimmedTeamSupply int64
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_trimmedSupply, err := service.GetTotalSupply()
		if err != nil {
			errCh <- errors.New("error while retrieving supply: " + err.Error())
			return
		}
		trimmedSupply = _trimmedSupply
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_trimmedMinted, err := service.GetTotalMintedAmount()
		if err != nil {
			errCh <- errors.New("error while retrieving minted amount: " + err.Error())
			return
		}
		trimmedMinted = _trimmedMinted
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_trimmedBurned, err := service.GetTotalBurnedAmount()
		if err != nil {
			errCh <- errors.New("error while retrieving burned amount: " + err.Error())
			return
		}
		trimmedBurned = _trimmedBurned
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_trimmedTeamSupply, err := service.GetTeamWalletsSupply()
		if err != nil {
			errCh <- errors.New("error while retrieving team supply: " + err.Error())
			return
		}
		trimmedTeamSupply = _trimmedTeamSupply
	}()
	wg.Wait()
	close(errCh)

	if len(errCh) > 0 {
		var errorMsgs []string
		for err := range errCh {
			errorMsgs = append(errorMsgs, err.Error())
		}
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, strings.Join(errorMsgs, " | "))
		return
	}

	response := tokenSupplyResponse{
		InitilaMinted:     0,
		MaxSupply:         161803398,
		TotalSupply:       trimmedSupply,
		CirculatingSupply: trimmedSupply - trimmedTeamSupply,
		Burned:            trimmedBurned,
		Minted:            trimmedMinted,
	}

	c.JSON(http.StatusOK, response)
}
