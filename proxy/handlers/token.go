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
	CirculatingSupply string `json:"circulatingSupply"`
	TotalSupply       string `json:"totalSupply"`
	MaxSupply         string `json:"maxSupply"`
	Minted            string `json:"minted"`
	Burned            string `json:"burned"`
	InitilaMinted     string `json:"initialMinted"`
	NodeAddress       string `json:"nodeAddress"`
	NdContractBurn    string `json:"ndContractBurn"`
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

			circulatingSupply := service.CalcCircSupply(service.GetAmountAsFloatString(teamSupply), service.GetAmountAsFloatString(totalSupply))
			c.String(http.StatusOK, "%d", circulatingSupply)
			return
		case "totalSupply":
			totalSupply, err := service.GetTotalSupply()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalSupply))
			return
		case "minted":
			totalMinted, err := service.GetTotalMintedAmount()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalMinted))
			return
		case "burned":
			totalBurned, err := service.GetTotalBurnedAmount()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalBurned))
			return
		case "ndBurned":
			ndContractBurn, err := service.GetNdContractTotalBurnedAmount()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(ndContractBurn))
			return
		case "teamSupply":
			teamSupply, err := service.GetTeamWalletsSupply()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}
			c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(teamSupply))
			return
		case "initailMinted":
			c.String(http.StatusOK, "%d", 0)
			return
		case "maxSupply":
			c.String(http.StatusOK, "%d", 161803398)
			return
		}
	}

	var trimmedSupply, trimmedMinted, trimmedBurned, trimmedTeamSupply, ndContractBurned string
	var wg sync.WaitGroup
	errCh := make(chan error, 4)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_trimmedSupply, err := service.GetTotalSupply()
		if err != nil {
			errCh <- errors.New("error while retrieving supply: " + err.Error())
			return
		}
		trimmedSupply = service.GetAmountAsFloatString(_trimmedSupply)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_trimmedMinted, err := service.GetTotalMintedAmount()
		if err != nil {
			errCh <- errors.New("error while retrieving minted amount: " + err.Error())
			return
		}
		trimmedMinted = service.GetAmountAsFloatString(_trimmedMinted)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_trimmedBurned, err := service.GetTotalBurnedAmount()
		if err != nil {
			errCh <- errors.New("error while retrieving burned amount: " + err.Error())
			return
		}
		trimmedBurned = service.GetAmountAsFloatString(_trimmedBurned)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_trimmedTeamSupply, err := service.GetTeamWalletsSupply()
		if err != nil {
			errCh <- errors.New("error while retrieving team supply: " + err.Error())
			return
		}
		trimmedTeamSupply = service.GetAmountAsFloatString(_trimmedTeamSupply)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ndContractBurned, err := service.GetNdContractTotalBurnedAmount()
		if err != nil {
			errCh <- errors.New("error while retrieving supply: " + err.Error())
			return
		}
		ndContractBurned = service.GetAmountAsFloatString(_ndContractBurned)
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
		InitilaMinted:     "0",
		MaxSupply:         "161803398",
		TotalSupply:       trimmedSupply,
		CirculatingSupply: service.CalcCircSupply(trimmedTeamSupply, trimmedSupply),
		Burned:            trimmedBurned,
		Minted:            trimmedMinted,
		NdContractBurn:    ndContractBurned,
		NodeAddress:       nodeAddress,
	}

	c.JSON(http.StatusOK, response)
}
