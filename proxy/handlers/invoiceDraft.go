package handlers

import (
	"net/http"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

const (
	invoiceDraftBaseEndpoint = "/invoice-draft"
	/* Node owner endpoints */
	getNodeOwnerDraftListEndpoint  = "/get-drafts"
	downloadNodeOwnerDraftEndpoint = "/download-draft"
	createPreferenceEndpoint       = "/create-preferences"
	changePreferencesEndpoint      = "/change-preferences"

	/* CSP endpoints */
	getCspDraftListEndpoint  = "/get-csp-drafts"
	downloadCspDraftEndpoint = "/download-csp-draft"
)

type getInvoiceDraftsRequest struct {
	DraftId           uuid.UUID `json:"invoiceId"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
	UserAddress       string    `json:"userAddress"`
	CspOwner          string    `json:"cspOwnerAddress"`
	TotalUsdcAmount   float64   `json:"totalUsdcAmount"`
	InvoiceSeries     string    `json:"invoiceSeries"`
	InvoiceNumber     int       `json:"invoiceNumber"`
	NodeOwnerName     string    `json:"nodeOwnerName"`
	CspOwnerName      string    `json:"cspOwnerName"`
}

type invoiceDraftHandler struct{}

func NewInvoiceDraftHandler(groupHandler *groupHandler) {
	h := &invoiceDraftHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: getNodeOwnerDraftListEndpoint, HandlerFunc: h.getNodeOnwerDraftList},
		{Method: http.MethodGet, Path: getCspDraftListEndpoint, HandlerFunc: h.getCspDraftList},
		{Method: http.MethodGet, Path: downloadNodeOwnerDraftEndpoint, HandlerFunc: h.downloadNodeOnwerDraft},
		{Method: http.MethodGet, Path: downloadCspDraftEndpoint, HandlerFunc: h.downloadCspDraft},

		{Method: http.MethodPost, Path: changePreferencesEndpoint, HandlerFunc: h.changePreferences},
		{Method: http.MethodPost, Path: createPreferenceEndpoint, HandlerFunc: h.createPreferences},
	}

	endpointGroupHandler := EndpointGroupHandler{
		Root:             invoiceDraftBaseEndpoint,
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

func (h *invoiceDraftHandler) getNodeOnwerDraftList(c *gin.Context) {
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

	drafts, err := storage.GetDraftListByNodeOwner(userAddress)
	if err != nil {
		log.Error("error while retrieving report: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	var parsedDraft []getInvoiceDraftsRequest

	userName, _ := drafts[0].UserProfile.GetNameAsString() //it's always the same
	for _, d := range drafts {
		cspName, _ := d.CspProfile.GetNameAsString()
		newParsedDraft := getInvoiceDraftsRequest{
			DraftId:           d.DraftId,
			CreationTimestamp: d.CreationTimestamp,
			UserAddress:       d.UserAddress,
			CspOwner:          d.CspOwner,
			TotalUsdcAmount:   d.TotalUsdcAmount,
			InvoiceSeries:     d.InvoiceSeries,
			InvoiceNumber:     d.InvoiceNumber,
			NodeOwnerName:     userName,
			CspOwnerName:      cspName,
		}
		parsedDraft = append(parsedDraft, newParsedDraft)
	}

	model.JsonResponse(c, http.StatusOK, parsedDraft, nodeAddress, "")
}

func (h *invoiceDraftHandler) getCspDraftList(c *gin.Context) {
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

	drafts, err := storage.GetDraftListByCSP(userAddress)
	if err != nil {
		log.Error("error while retrieving report: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	var parsedDraft []getInvoiceDraftsRequest
	cspName, _ := drafts[0].CspProfile.GetNameAsString() //it's always the same
	for _, d := range drafts {
		userName, _ := d.UserProfile.GetNameAsString()
		newParsedDraft := getInvoiceDraftsRequest{
			DraftId:           d.DraftId,
			CreationTimestamp: d.CreationTimestamp,
			UserAddress:       d.UserAddress,
			CspOwner:          d.CspOwner,
			TotalUsdcAmount:   d.TotalUsdcAmount,
			InvoiceSeries:     d.InvoiceSeries,
			InvoiceNumber:     d.InvoiceNumber,
			NodeOwnerName:     userName,
			CspOwnerName:      cspName,
		}
		parsedDraft = append(parsedDraft, newParsedDraft)
	}

	model.JsonResponse(c, http.StatusOK, parsedDraft, nodeAddress, "")
}

func (h *invoiceDraftHandler) downloadNodeOnwerDraft(c *gin.Context) {
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

	draftId, ok := c.GetQuery("draftId")
	if !ok || draftId == "" {
		log.Error("draft id not received")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "draft id not received")
		return
	}

	drafts, err := storage.GetDraftByReportId(draftId, userAddress)
	if err != nil {
		log.Error("error while retrieving report: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	allocations, err := storage.GetAllocationsByDraftId(draftId)
	if err != nil {
		log.Error("error while retrieving allocations: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	byteFile, err := service.GenerateInvoiceDOC(*drafts, allocations)
	if err != nil {
		log.Error("error while generating invoice doc: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename=invoice_draft.doc")
	c.Data(http.StatusOK, "application/msword", byteFile)
}

func (h *invoiceDraftHandler) downloadCspDraft(c *gin.Context) {
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

	draftId, ok := c.GetQuery("draftId")
	if !ok || draftId == "" {
		log.Error("draft id not received")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "draft id not received")
		return
	}

	drafts, err := storage.GetCspDraftByReportId(draftId, userAddress)
	if err != nil {
		log.Error("error while retrieving report: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	allocations, err := storage.GetAllocationsByDraftId(draftId)
	if err != nil {
		log.Error("error while retrieving allocations: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	byteFile, err := service.GenerateInvoiceDOC(*drafts, allocations)
	if err != nil {
		log.Error("error while generating invoice doc: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename=invoice_draft.doc")
	c.Data(http.StatusOK, "application/msword", byteFile)
}

/*
.########...#######...######..########
.##.....##.##.....##.##....##....##...
.##.....##.##.....##.##..........##...
.########..##.....##..######.....##...
.##........##.....##.......##....##...
.##........##.....##.##....##....##...
.##.........#######...######.....##...
*/

func (h *invoiceDraftHandler) createPreferences(c *gin.Context) {
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

	var pref model.Preference
	if err := c.ShouldBindJSON(&pref); err != nil {
		log.Error("error while binding json: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "error while binding json: "+err.Error())
		return
	}

	if pref.UserAddress != "" {
		log.Error("preference user address must be empty")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "preference user address must be empty")
		return
	}
	pref.UserAddress = userAddress

	err = storage.CreatePreference(&pref)
	if err != nil {
		log.Error("error while updating preference: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
}

func (h *invoiceDraftHandler) changePreferences(c *gin.Context) {
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

	var pref model.Preference
	if err := c.ShouldBindJSON(&pref); err != nil {
		log.Error("error while binding json: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "error while binding json: "+err.Error())
		return
	}

	if pref.UserAddress != userAddress {
		log.Error("preference user address does not match bearer address")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "preference user address does not match bearer address")
		return
	}

	err = storage.UpdatePreference(&pref)
	if err != nil {
		log.Error("error while updating preference: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
}
