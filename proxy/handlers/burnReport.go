package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"

	"github.com/gin-gonic/gin"
)

const (
	burnReportBaseEndpoint        = "/burn-report"
	getCspBurnReportEndpoint      = "/get-burn-report"
	downloadCspBurnReportEndpoint = "/download-burn-report"
)

type getBurnReportsResponse struct {
	TotalBurnedEventFound int                `json:"totalBurnedEventFound"`
	BurnReports           []burnReportStruct `json:"burnReports"`
}

type burnReportStruct struct {
	CreationTimestamp time.Time `json:"creationTimestamp"`
	TotalUsdcAmount   float64   `json:"totalUsdcAmount"`
	TotalR1Amount     float64   `json:"totalR1Amount"`
}

type burnReportHandler struct{}

func NewBurnReportHandler(groupHandler *groupHandler) {
	h := &burnReportHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: getCspBurnReportEndpoint, HandlerFunc: h.getBurnRepoort},
		{Method: http.MethodGet, Path: downloadCspBurnReportEndpoint, HandlerFunc: h.downloadBurnReport},
	}

	endpointGroupHandler := EndpointGroupHandler{
		Root:             burnReportBaseEndpoint,
		Middleware:       []gin.HandlerFunc{middleware.Authorization(config.Config.Jwt.Secret)},
		EndpointHandlers: endpoints,
	}

	groupHandler.AddEndpointGroupHandler(endpointGroupHandler)
}

/*
..######...########.########
.##....##..##..........##...
.##........##..........##...
.##...####.######......##...
.##....##..##..........##...
.##....##..##..........##...
..######...########....##...
*/

func (h *burnReportHandler) getBurnRepoort(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	userAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	startPosInt, pageDimInt := 0, 50
	startPos := c.Query("startPos")
	if startPos != "" {
		startPosInt, err = strconv.Atoi(startPos)
		if err != nil {
			log.Error("error while parsing startPos: " + err.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "error while parsing startPos: "+err.Error())
			return
		} else if startPosInt < 0 {
			startPosInt = 0
		}
	}

	pageDim := c.Query("pageDim")
	if pageDim != "" {
		pageDimInt, err = strconv.Atoi(pageDim)
		if err != nil {
			log.Error("error while parsing pageDim: " + err.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "error while parsing pageDim: "+err.Error())
			return
		} else if pageDimInt <= 0 {
			pageDimInt = 50
		}
	}

	if config.Config.Api.DevTesting {
		service.BuildMocks()
		b := service.GetMockBurnEvents()
		burnReport := []burnReportStruct{}
		eventRequested := b[startPosInt : startPosInt+pageDimInt]
		for _, b := range eventRequested {
			newBurnreport := burnReportStruct{
				CreationTimestamp: b.BurnTimestamp,
				TotalUsdcAmount:   service.GetAmountAsFloat(b.GetUsdcAmountSwapped(), model.UsdcDecimals),
				TotalR1Amount:     service.GetAmountAsFloat(b.GetR1AmountBurned(), model.R1Decimals),
			}
			burnReport = append(burnReport, newBurnreport)
		}
		response := getBurnReportsResponse{
			TotalBurnedEventFound: len(b),
			BurnReports:           burnReport,
		}
		model.JsonResponse(c, http.StatusOK, response, nodeAddress, "")
	}

	burnEvents, err := storage.GetBurnEventsByOwnerAddress(userAddress)
	if err != nil {
		log.Error("error while retrieving report: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	if len(burnEvents) == 0 {
		model.JsonResponse(c, http.StatusOK, burnEvents, nodeAddress, "")
		return
	}

	burnReport := []burnReportStruct{}
	eventRequested := burnEvents[startPosInt : startPosInt+pageDimInt]
	for _, b := range eventRequested {
		newBurnreport := burnReportStruct{
			CreationTimestamp: b.BurnTimestamp,
			TotalUsdcAmount:   service.GetAmountAsFloat(b.GetUsdcAmountSwapped(), model.UsdcDecimals),
			TotalR1Amount:     service.GetAmountAsFloat(b.GetR1AmountBurned(), model.R1Decimals),
		}
		burnReport = append(burnReport, newBurnreport)
	}

	response := getBurnReportsResponse{
		TotalBurnedEventFound: len(burnEvents),
		BurnReports:           burnReport,
	}

	model.JsonResponse(c, http.StatusOK, response, nodeAddress, "")
}

func (h *burnReportHandler) downloadBurnReport(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	userAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	startTimeStr := c.Query("startTime")
	if startTimeStr == "" {
		log.Error("startTime query param is missing")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "startTime query param is missing")
		return
	}

	endTimeStr := c.Query("endTime")
	if endTimeStr == "" {
		log.Error("endTime query param is missing")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "endTime query param is missing")
		return
	}

	layout := "02-01-2006"
	startTime, err := time.Parse(layout, startTimeStr)
	if err != nil {
		log.Error("invalid startTime format: %v", err)
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "invalid startTime format, expected DD-MM-YYYY")
		return
	}

	endTime, err := time.Parse(layout, endTimeStr)
	if err != nil {
		log.Error("invalid endTime format: %v", err)
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "invalid endTime format, expected DD-MM-YYYY")
		return
	}

	if config.Config.Api.DevTesting {
		service.BuildMocks()
		b := service.GetMockBurnEvents()
		requestedBurnEvents := []model.BurnEvent{}
		for _, be := range b {
			if be.BurnTimestamp.After(startTime) && be.BurnTimestamp.Before(endTime) {
				requestedBurnEvents = append(requestedBurnEvents, be)
			}
		}

		byteFile, err := service.GenerateBurnReportCSV(requestedBurnEvents)
		if err != nil {
			log.Error("error while generating invoice doc: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
		c.Header("Content-Disposition", "attachment; filename=invoice_draft.doc")
		c.Data(http.StatusOK, "application/msword", byteFile)
		return
	}

	burnEvents, err := storage.GetBurnEventsForUserInTimeRange(startTime, endTime, userAddress)
	if err != nil {
		log.Error("error while retrieving report: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	if len(burnEvents) == 0 {
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "no burn event for that period")
		return
	}

	byteFile, err := service.GenerateBurnReportCSV(burnEvents)
	if err != nil {
		log.Error("error while generating invoice doc: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename=invoice_draft.doc")
	c.Data(http.StatusOK, "application/msword", byteFile)
}
