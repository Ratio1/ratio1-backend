package handlers

import (
	"errors"
	"math/big"
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

var oneToken = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)

type tokenSupplyResponse struct {
	CirculatingSupply any    `json:"circulatingSupply"`
	TotalSupply       any    `json:"totalSupply"`
	MaxSupply         any    `json:"maxSupply"`
	Minted            any    `json:"minted"`
	Burned            any    `json:"burned"`
	InitilaMinted     any    `json:"initialMinted"`
	NdContractBurn    any    `json:"ndContractBurn"`
	NodeAddress       string `json:"nodeAddress"`
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

			trimmedTotalSupply := big.NewInt(0).Div(totalSupply, oneToken)
			trimmedTeamSupply := big.NewInt(0).Div(teamSupply, oneToken)
			circulatingSupply := trimmedTotalSupply.Int64() - trimmedTeamSupply.Int64()
			//circulatingSupply := service.CalcCircSupply(service.GetAmountAsFloatString(teamSupply), service.GetAmountAsFloatString(totalSupply))
			c.String(http.StatusOK, "%d", circulatingSupply)
			return
		case "totalSupply":
			totalSupply, err := service.GetTotalSupply()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedTotalSupply := big.NewInt(0).Div(totalSupply, oneToken)
			c.String(http.StatusOK, "%d", trimmedTotalSupply.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalSupply))
			return
		case "minted":
			totalMinted, err := service.GetTotalMintedAmount()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedMinted := big.NewInt(0).Div(totalMinted, oneToken)
			c.String(http.StatusOK, "%d", trimmedMinted.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalMinted))
			return
		case "burned":
			totalBurned, err := service.GetTotalBurnedAmount()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedBurned := big.NewInt(0).Div(totalBurned, oneToken)
			c.String(http.StatusOK, "%d", trimmedBurned.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalBurned))
			return
		case "ndBurned":
			ndContractBurn, err := service.GetNdContractTotalBurnedAmount()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedNdContractBurn := big.NewInt(0).Div(ndContractBurn, oneToken)
			c.String(http.StatusOK, "%d", trimmedNdContractBurn.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(ndContractBurn))
			return
		case "teamSupply":
			teamSupply, err := service.GetTeamWalletsSupply()
			if err != nil {
				model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
				return
			}

			trimmedTeamSupply := big.NewInt(0).Div(teamSupply, oneToken)
			c.String(http.StatusOK, "%d", trimmedTeamSupply.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(teamSupply))
			return
		case "initailMinted":
			c.String(http.StatusOK, "%d", 0)
			return
		case "maxSupply":
			c.String(http.StatusOK, "%d", 161803398)
			return
		}
	}

	var totalSupplyString, totalMintedString, totalBurnedString, teamSupplyString, ndContractBurnedString string //with decimals
	var trimmedSupplyInt, trimmedMintedInt, trimmedBurnedInt, trimmedTeamSupplyInt, ndContractBurnedInt int64    //without decimals
	var wg sync.WaitGroup
	errCh := make(chan error, 4)
	wg.Add(1)
	go func() {
		defer wg.Done()
		totalSupply, err := service.GetTotalSupply()
		if err != nil {
			errCh <- errors.New("error while retrieving supply: " + err.Error())
			return
		}
		trimmedSupplyInt = big.NewInt(0).Div(totalSupply, oneToken).Int64()
		totalSupplyString = service.GetAmountAsFloatString(totalSupply)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		totalMinted, err := service.GetTotalMintedAmount()
		if err != nil {
			errCh <- errors.New("error while retrieving minted amount: " + err.Error())
			return
		}
		trimmedMintedInt = big.NewInt(0).Div(totalMinted, oneToken).Int64()
		totalMintedString = service.GetAmountAsFloatString(totalMinted)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		totalBurned, err := service.GetTotalBurnedAmount()
		if err != nil {
			errCh <- errors.New("error while retrieving burned amount: " + err.Error())
			return
		}
		trimmedBurnedInt = big.NewInt(0).Div(totalBurned, oneToken).Int64()
		totalBurnedString = service.GetAmountAsFloatString(totalBurned)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		teamSupply, err := service.GetTeamWalletsSupply()
		if err != nil {
			errCh <- errors.New("error while retrieving team supply: " + err.Error())
			return
		}
		trimmedTeamSupplyInt = big.NewInt(0).Div(teamSupply, oneToken).Int64()
		teamSupplyString = service.GetAmountAsFloatString(teamSupply)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		ndContractBurned, err := service.GetNdContractTotalBurnedAmount()
		if err != nil {
			errCh <- errors.New("error while retrieving supply: " + err.Error())
			return
		}
		ndContractBurnedInt = big.NewInt(0).Div(ndContractBurned, oneToken).Int64()
		ndContractBurnedString = service.GetAmountAsFloatString(ndContractBurned)
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
	var response tokenSupplyResponse
	withDecimals, ok := c.GetQuery("withDecimals")
	if ok && withDecimals == "true" {
		response = tokenSupplyResponse{
			InitilaMinted:     "0",
			MaxSupply:         "161803398",
			TotalSupply:       totalSupplyString,
			CirculatingSupply: service.CalcCircSupply(teamSupplyString, totalSupplyString),
			Burned:            totalBurnedString,
			Minted:            totalMintedString,
			NdContractBurn:    ndContractBurnedString,
			NodeAddress:       nodeAddress,
		}
	} else {
		response = tokenSupplyResponse{
			InitilaMinted:     0,
			MaxSupply:         161803398,
			TotalSupply:       trimmedSupplyInt,
			CirculatingSupply: trimmedSupplyInt - trimmedTeamSupplyInt,
			Burned:            trimmedBurnedInt,
			Minted:            trimmedMintedInt,
			NdContractBurn:    ndContractBurnedInt,
			NodeAddress:       nodeAddress,
		}
	}

	c.JSON(http.StatusOK, response)
}
