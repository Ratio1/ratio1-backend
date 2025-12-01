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
	getNodeOwnerDraftListEndpoint      = "/get-drafts"
	downloadNodeOwnerDraftEndpoint     = "/download-draft"
	downloadNodeOwnerDraftJSONEndpoint = "/download-draft-json"
	createPreferenceEndpoint           = "/create-preferences"
	changePreferencesEndpoint          = "/change-preferences"
	getPreferencesEndpoint             = "/get-preferences"

	/* CSP endpoints */
	getCspDraftListEndpoint      = "/get-csp-drafts"
	downloadCspDraftEndpoint     = "/download-csp-draft"
	downloadCspDraftJSONEndpoint = "/download-csp-draft-json"
)

type getInvoiceDraftsResponse struct {
	DraftId           uuid.UUID `json:"draftId"`
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
		{Method: http.MethodGet, Path: getNodeOwnerDraftListEndpoint, HandlerFunc: h.getNodeOwnerDraftList},
		{Method: http.MethodGet, Path: getCspDraftListEndpoint, HandlerFunc: h.getCspDraftList},
		{Method: http.MethodGet, Path: getPreferencesEndpoint, HandlerFunc: h.getPreferences},
		{Method: http.MethodGet, Path: downloadNodeOwnerDraftEndpoint, HandlerFunc: h.downloadNodeOwnerDraft},
		{Method: http.MethodGet, Path: downloadNodeOwnerDraftJSONEndpoint, HandlerFunc: h.downloadNodeOwnerDraftJSON},
		{Method: http.MethodGet, Path: downloadCspDraftEndpoint, HandlerFunc: h.downloadCspDraft},
		{Method: http.MethodGet, Path: downloadCspDraftJSONEndpoint, HandlerFunc: h.downloadCspDraftJSON},

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

func (h *invoiceDraftHandler) getNodeOwnerDraftList(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	if config.Config.Api.DevTesting {
		service.BuildMocks()
		i, _ := service.GetMockOperatorData()
		userName, _ := i[0].UserProfile.GetNameAsString()
		var parsedDraft []getInvoiceDraftsResponse
		for _, d := range i {
			if d.UserAddress == d.CspOwner {
				continue
			}
			cspName, _ := d.CspProfile.GetNameAsString()
			newParsedDraft := getInvoiceDraftsResponse{
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

	parsedDraft := []getInvoiceDraftsResponse{}
	if len(drafts) == 0 {
		model.JsonResponse(c, http.StatusOK, parsedDraft, nodeAddress, "")
		return
	}
	userName, _ := drafts[0].UserProfile.GetNameAsString() //it's always the same
	for _, d := range drafts {
		if d.UserAddress == d.CspOwner {
			continue
		}
		cspName, _ := d.CspProfile.GetNameAsString()
		newParsedDraft := getInvoiceDraftsResponse{
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

	if config.Config.Api.DevTesting {
		service.BuildMocks()
		i, _ := service.GetMockCspData()
		var parsedDraft []getInvoiceDraftsResponse
		cspName, _ := i[0].CspProfile.GetNameAsString() //it's always the same
		for _, d := range i {
			if d.UserAddress == d.CspOwner {
				continue
			}
			userName, _ := d.UserProfile.GetNameAsString()
			newParsedDraft := getInvoiceDraftsResponse{
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

	parsedDraft := []getInvoiceDraftsResponse{}
	if len(drafts) == 0 {
		model.JsonResponse(c, http.StatusOK, parsedDraft, nodeAddress, "")
		return
	}
	cspName, _ := drafts[0].CspProfile.GetNameAsString() //it's always the same
	for _, d := range drafts {
		if d.UserAddress == d.CspOwner {
			continue
		}
		userName, _ := d.UserProfile.GetNameAsString()
		newParsedDraft := getInvoiceDraftsResponse{
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

func (h *invoiceDraftHandler) downloadNodeOwnerDraft(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	draftId, ok := c.GetQuery("draftId")
	if !ok || draftId == "" {
		log.Error("draft id not received")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "draft id not received")
		return
	}

	if config.Config.Api.DevTesting {
		service.BuildMocks()
		i, a := service.GetMockOperatorData()
		var invoice model.InvoiceDraft
		found := false
		for _, v := range i {
			if v.DraftId.String() == draftId {
				found = true
				invoice = v
			}
		}
		if !found {
			log.Error("draft id not found in storage")
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "draft id not found in storage")
			return
		}
		byteFile, err := service.FillInvoiceDraftTemplate(invoice, a)
		if err != nil {
			log.Error("error while generating invoice doc: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
		c.Header("Content-Disposition", "attachment; filename=invoice_draft.doc")
		c.Data(http.StatusOK, "application/msword", byteFile)
		return
	}

	userAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
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

	byteFile, err := service.FillInvoiceDraftTemplate(*drafts, allocations)
	if err != nil {
		log.Error("error while generating invoice doc: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename=invoice_draft.doc")
	c.Data(http.StatusOK, "application/msword", byteFile)
}

func (h *invoiceDraftHandler) downloadNodeOwnerDraftJSON(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	draftId, ok := c.GetQuery("draftId")
	if !ok || draftId == "" {
		log.Error("draft id not received")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "draft id not received")
		return
	}

	if config.Config.Api.DevTesting {
		service.BuildMocks()
		i, a := service.GetMockOperatorData()
		var invoice model.InvoiceDraft
		found := false
		for _, v := range i {
			if v.DraftId.String() == draftId {
				found = true
				invoice = v
			}
		}
		if !found {
			log.Error("draft id not found in storage")
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "draft id not found in storage")
			return
		}
		invoiceStruct, err := service.FillInvoiceDraftTemplateJSON(invoice, a)
		if err != nil {
			log.Error("error while generating invoice doc: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
		model.JsonResponse(c, http.StatusOK, invoiceStruct, nodeAddress, "")
		return
	}

	userAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
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

	invoiceStruct, err := service.FillInvoiceDraftTemplateJSON(*drafts, allocations)
	if err != nil {
		log.Error("error while generating invoice doc: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}
	model.JsonResponse(c, http.StatusOK, invoiceStruct, nodeAddress, "")
}

func (h *invoiceDraftHandler) downloadCspDraft(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	draftId, ok := c.GetQuery("draftId")
	if !ok || draftId == "" {
		log.Error("draft id not received")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "draft id not received")
		return
	}

	if config.Config.Api.DevTesting {
		service.BuildMocks()
		i, a := service.GetMockCspData()

		var invoice model.InvoiceDraft
		found := false
		for _, v := range i {
			if v.DraftId.String() == draftId {
				found = true
				invoice = v
			}
		}
		if !found {
			log.Error("draft id not found in storage")
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "draft id not found in storage")
			return
		}

		byteFile, err := service.FillInvoiceDraftTemplate(invoice, a)
		if err != nil {
			log.Error("error while generating invoice doc: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
		c.Header("Content-Disposition", "attachment; filename=invoice_draft.doc")
		c.Data(http.StatusOK, "application/msword", byteFile)
		return
	}

	userAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
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

	byteFile, err := service.FillInvoiceDraftTemplate(*drafts, allocations)
	if err != nil {
		log.Error("error while generating invoice doc: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename=invoice_draft.doc")
	c.Data(http.StatusOK, "application/msword", byteFile)
}

func (h *invoiceDraftHandler) downloadCspDraftJSON(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	draftId, ok := c.GetQuery("draftId")
	if !ok || draftId == "" {
		log.Error("draft id not received")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "draft id not received")
		return
	}

	if config.Config.Api.DevTesting {
		service.BuildMocks()
		i, a := service.GetMockCspData()

		var invoice model.InvoiceDraft
		found := false
		for _, v := range i {
			if v.DraftId.String() == draftId {
				found = true
				invoice = v
			}
		}
		if !found {
			log.Error("draft id not found in storage")
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "draft id not found in storage")
			return
		}

		invoiceStruct, err := service.FillInvoiceDraftTemplateJSON(invoice, a)
		if err != nil {
			log.Error("error while generating invoice doc: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
		model.JsonResponse(c, http.StatusOK, invoiceStruct, nodeAddress, "")
		return
	}

	userAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
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

	invoiceStruct, err := service.FillInvoiceDraftTemplateJSON(*drafts, allocations)
	if err != nil {
		log.Error("error while generating invoice doc: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}
	model.JsonResponse(c, http.StatusOK, invoiceStruct, nodeAddress, "")
}

func (h *invoiceDraftHandler) getPreferences(c *gin.Context) {
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

	preference, err := storage.GetPreferenceByAddress(userAddress)
	if err != nil {
		log.Error("error while retrieving report: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, preference, nodeAddress, "")
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
