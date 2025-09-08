package handlers

import (
	"math/big"
	"net/http"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
)

const (
	baseTokenEndpoint = "/token"
	getSupplyEndpoint = "/supply"
	getStatsEncpoint  = "/stats"
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
		{Method: http.MethodGet, Path: getStatsEncpoint, HandlerFunc: h.getStats},
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

	stats, err := storage.GetLatestStats()
	if err != nil {
		log.Error("error while retrieving latest stats from db: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving latest stats from db: "+err.Error())
		return
	} else if stats == nil {
		log.Error("no stats found in db")
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "no stats found in db")
		return
	}

	token, ok := c.GetQuery("extract")
	if ok && token != "" {
		switch token {
		case "circulatingSupply":
			trimmedTotalSupply := big.NewInt(0).Div(stats.TotalSupply, oneToken)
			trimmedTeamSupply := big.NewInt(0).Div(stats.TeamWalletsSupply, oneToken)
			circulatingSupply := trimmedTotalSupply.Int64() - trimmedTeamSupply.Int64()
			//circulatingSupply := service.CalcCircSupply(service.GetAmountAsFloatString(teamSupply), service.GetAmountAsFloatString(totalSupply))
			c.String(http.StatusOK, "%d", circulatingSupply)
			return
		case "totalSupply":
			trimmedTotalSupply := big.NewInt(0).Div(stats.TotalSupply, oneToken)
			c.String(http.StatusOK, "%d", trimmedTotalSupply.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalSupply))
			return
		case "minted":
			trimmedMinted := big.NewInt(0).Div(stats.TotalMinted, oneToken)
			c.String(http.StatusOK, "%d", trimmedMinted.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalMinted))
			return
		case "burned":
			trimmedBurned := big.NewInt(0).Div(stats.TotalTokenBurn, oneToken)
			c.String(http.StatusOK, "%d", trimmedBurned.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(totalBurned))
			return
		case "ndBurned":
			trimmedNdContractBurn := big.NewInt(0).Div(stats.TotalNdContractTokenBurn, oneToken)
			c.String(http.StatusOK, "%d", trimmedNdContractBurn.Int64())
			//c.String(http.StatusOK, "%d", service.GetAmountAsFloatString(ndContractBurn))
			return
		case "teamSupply":
			trimmedTeamSupply := big.NewInt(0).Div(stats.TeamWalletsSupply, oneToken)
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

	trimmedSupplyInt := big.NewInt(0).Div(stats.TotalSupply, oneToken).Int64()
	totalSupplyString := service.GetAmountAsFloatString(stats.TotalSupply, model.R1Decimals)

	trimmedMintedInt := big.NewInt(0).Div(stats.DailyMinted, oneToken).Int64()
	totalMintedString := service.GetAmountAsFloatString(stats.DailyMinted, model.R1Decimals)

	trimmedBurnedInt := big.NewInt(0).Div(stats.DailyTokenBurn, oneToken).Int64()
	totalBurnedString := service.GetAmountAsFloatString(stats.DailyTokenBurn, model.R1Decimals)

	trimmedTeamSupplyInt := big.NewInt(0).Div(stats.TeamWalletsSupply, oneToken).Int64()
	teamSupplyString := service.GetAmountAsFloatString(stats.TeamWalletsSupply, model.R1Decimals)

	ndContractBurnedInt := big.NewInt(0).Div(stats.DailyNdContractTokenBurn, oneToken).Int64()
	ndContractBurnedString := service.GetAmountAsFloatString(stats.DailyNdContractTokenBurn, model.R1Decimals)

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

func (h *tokenHandler) getStats(c *gin.Context) {
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

	stats, err := storage.GetAllStatsASC()
	if err != nil {
		log.Error("error while retrieving stats from db: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving stats from db: "+err.Error())
		return
	} else if stats == nil {
		log.Error("no stats found in db")
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "no stats found in db")
		return
	}

	model.JsonResponse(c, http.StatusOK, stats, nodeAddress, "")
}
