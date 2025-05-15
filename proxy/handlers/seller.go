package handlers

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
)

const (
	baseSellerEndpoint = "/seller"
	newSellerEndpoint  = "/new"
	getSellerClients   = "/clients"
	//TODO add admins seller get clients
	getSellerCode = "/code"
)

type newSellerRequest struct {
	Address    string `json:"address" binding:"required"`
	ForcesCode string `json:"forcedCode"`
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

	/*  TODO check if user is valid(?)
	userAddress, err := middleware.AddressFromBearer(c)
	if err != nil {
		log.Error("error while retrieving address from bearer: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}*/

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
	if newSellerRequest.ForcesCode != "" {
		newCode = newSellerRequest.ForcesCode
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

	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
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

/*
.##.....##.########.####.##........######.
.##.....##....##.....##..##.......##....##
.##.....##....##.....##..##.......##......
.##.....##....##.....##..##........######.
.##.....##....##.....##..##.............##
.##.....##....##.....##..##.......##....##
..#######.....##....####.########..######.
*/

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateCode(length int) string {
	rand.Seed(time.Now().UnixNano())
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return string(code)
}
