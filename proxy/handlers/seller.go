package handlers

import (
	"math/rand"
	"net/http"
	"slices"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
)

const (
	baseSellerEndpoint        = "/seller"
	newSellerEndpoint         = "/new"
	getSellerClients          = "/clients"
	getAllSellerCodes         = "/all-codes"
	enableSellerCodeEndpoint  = "/enable"
	disableSellerCodeEndpoint = "/disable"
	getSellerCode             = "/code"
)

type newSellerRequest struct {
	Address    string `json:"address" binding:"required"`
	ForcedCode string `json:"forcedCode"`
}

type sellerClientsResponse struct {
	Address        string `json:"address"`
	LicensesNumber int    `json:"licensesNumber"`
	TotalValue     int    `json:"totalValue"`
}

type sellerHandler struct{}

func NewSellerHandler(groupHandler *groupHandler) {
	h := sellerHandler{}

	authEndpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: newSellerEndpoint, HandlerFunc: h.newSeller},
		{Method: http.MethodGet, Path: getSellerClients, HandlerFunc: h.getClients},
		{Method: http.MethodGet, Path: getSellerCode, HandlerFunc: h.getSellerCode},
		{Method: http.MethodGet, Path: getAllSellerCodes, HandlerFunc: h.getSellersCode},
		{Method: http.MethodPost, Path: disableSellerCodeEndpoint, HandlerFunc: h.disableSellerCode},
		{Method: http.MethodPost, Path: enableSellerCodeEndpoint, HandlerFunc: h.enableSellerCode},
	}

	auth := middleware.Authorization(config.Config.Jwt.Secret)
	authEndpointGroupHandler := EndpointGroupHandler{
		Root:             baseSellerEndpoint,
		Middleware:       []gin.HandlerFunc{auth},
		EndpointHandlers: authEndpoints,
	}

	groupHandler.AddEndpointGroupHandler(authEndpointGroupHandler)
}

func (h *sellerHandler) newSeller(c *gin.Context) {
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

	if !slices.Contains(config.Config.AdminAddresses, userAddress) {
		log.Error("user: " + userAddress + " is not an admin")
		model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "user is not an admin")
		return
	}

	var newSellerRequest newSellerRequest
	if err := c.ShouldBindJSON(&newSellerRequest); err != nil {
		log.Error("error while binding json: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "error while binding json: "+err.Error())
		return
	}

	address := newSellerRequest.Address
	if address == "" {
		log.Error("address is empty")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "address is empty")
		return
	}

	account, err := service.GetOrCreateAccount(address)
	if err != nil {
		log.Error("error while retrieving account information: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "error while retrieving account information: "+err.Error())
		return
	}

	if account.IsBlacklisted {
		if account.BlacklistedReason != nil {
			log.Error("account: " + address + " is blacklisted with reason: " + *account.BlacklistedReason)
			model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "account is blacklisted with reason:"+*account.BlacklistedReason)
			return
		} else {
			log.Error("account: " + address + " is blacklisted!")
			model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "account is blacklisted")
			return
		}
	}

	ok, err := storage.AddressHasCode(account.Address)
	if err != nil {
		log.Error("error while checking if address has code: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while checking if address has code: "+err.Error())
		return
	}
	if ok {
		log.Error("address: " + address + " already has a code")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "address already has a code")
		return
	}

	newCode := generateCode(6)
	if newSellerRequest.ForcedCode != "" {
		newCode = newSellerRequest.ForcedCode
	}

	ok, err = storage.SellerCodeDoExist(newCode)
	if err != nil {
		log.Error("error while checking if seller code already exists: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while checking if seller code already exists: "+err.Error())
		return
	}
	if ok {
		log.Error("code: " + newCode + " already exists")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "code already exists")
		return
	}

	err = storage.CreateSeller(&model.Seller{
		SellerCode: newCode,
		AccountID:  account.Address})
	if err != nil {
		log.Error("error while creating seller: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while creating seller: "+err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, newCode, nodeAddress, "")
}

func (h *sellerHandler) getClients(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	address, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	sellerCode, err := storage.GetSellerCodeByAddress(address)
	if err != nil {
		log.Error("error while retrieving seller code: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving seller code: "+err.Error())
		return
	}

	if sellerCode == nil {
		log.Error("address: " + address + " does not have a seller code")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "address does not have a seller code")
		return
	}

	users, err := storage.GetAccountsBySellerCode(*sellerCode)
	if err != nil {
		log.Error("error while retrieving users: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving users: "+err.Error())
		return
	}
	var response []sellerClientsResponse
	for _, u := range *users {

		invoices, err := storage.GetUserInvoices(u.Address)
		if err != nil {
			log.Error("error while retrieving invoices: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving invoices: "+err.Error())
			return
		}

		licensesNumber := 0
		totalValue := 0
		for _, invoice := range *invoices {
			licensesNumber += *invoice.NumLicenses
			totalValue += *invoice.UnitUsdPrice * *invoice.NumLicenses
		}

		response = append(response, sellerClientsResponse{
			Address:        u.Address,
			LicensesNumber: licensesNumber,
			TotalValue:     totalValue,
		})
	}

	model.JsonResponse(c, http.StatusOK, response, nodeAddress, "")
}

func (h *sellerHandler) getSellerCode(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	address, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	sellerCode, err := storage.GetSellerCodeByAddress(address)
	if err != nil {
		log.Error("error while retrieving seller code: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving seller code: "+err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, sellerCode, nodeAddress, "")
}

func (h *sellerHandler) getSellersCode(c *gin.Context) {
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

	if !slices.Contains(config.Config.AdminAddresses, userAddress) {
		log.Error("user: " + userAddress + " is not an admin")
		model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "user is not an admin")
		return
	}

	sellers, err := storage.GetAllSellerCode()
	if err != nil {
		log.Error("error while retrieving all seller codes: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving all seller codes: "+err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, sellers, nodeAddress, "")
}

func (h *sellerHandler) disableSellerCode(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	adminAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	if !slices.Contains(config.Config.AdminAddresses, adminAddress) {
		log.Error("user: " + adminAddress + " is not an admin")
		model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "user is not an admin")
		return
	}

	var seller *model.Seller
	userAddress, ok := c.GetQuery("userAddress")
	if ok && userAddress != "" {
		seller, err = storage.GetSellerByAddress(userAddress)
		if err != nil {
			log.Error("error while retrieving seller by address: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving seller by address: "+err.Error())
			return
		}
	} else {
		sellerCode, ok := c.GetQuery("sellerCode")
		if !ok || sellerCode == "" {
			log.Error("seller code is required")
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "seller code is required")
			return
		}
		seller, err = storage.GetSellerByCode(sellerCode)
		if err != nil {
			log.Error("error while retrieving seller by code: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving seller by code: "+err.Error())
			return
		}
	}

	if seller == nil {
		log.Error("seller not found")
		model.JsonResponse(c, http.StatusNotFound, nil, nodeAddress, "seller not found")
		return
	}

	seller.IsDisabled = true
	err = storage.UpdateSeller(seller)
	if err != nil {
		log.Error("error while updating seller: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while updating seller: "+err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
}

func (h *sellerHandler) enableSellerCode(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	adminAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	if !slices.Contains(config.Config.AdminAddresses, adminAddress) {
		log.Error("user: " + adminAddress + " is not an admin")
		model.JsonResponse(c, http.StatusUnauthorized, nil, nodeAddress, "user is not an admin")
		return
	}

	var seller *model.Seller
	userAddress, ok := c.GetQuery("userAddress")
	if ok && userAddress != "" {
		seller, err = storage.GetSellerByAddress(userAddress)
		if err != nil {
			log.Error("error while retrieving seller by address: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving seller by address: "+err.Error())
			return
		}
	} else {
		sellerCode, ok := c.GetQuery("sellerCode")
		if !ok || sellerCode == "" {
			log.Error("seller code is required")
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "seller code is required")
			return
		}
		seller, err = storage.GetSellerByCode(sellerCode)
		if err != nil {
			log.Error("error while retrieving seller by code: " + err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while retrieving seller by code: "+err.Error())
			return
		}
	}

	if seller == nil {
		log.Error("seller not found")
		model.JsonResponse(c, http.StatusNotFound, nil, nodeAddress, "seller not found")
		return
	}

	seller.IsDisabled = false
	err = storage.UpdateSeller(seller)
	if err != nil {
		log.Error("error while updating seller: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, "error while updating seller: "+err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
}

/*
.##.....##.########.####.##........######.
.##.....##....##.....##..##.......##....##
.##.....##....##.....##..##.......##......
.##.....##....##.....##..##........######.
.##.....##....##.....##..##.............##
.##.....##....##.....##..##.......##....##
..#######.....##....####.########..######.
*/

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateCode(length int) string {
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return string(code)
}
