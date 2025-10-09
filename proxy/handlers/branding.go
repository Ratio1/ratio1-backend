package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
)

const (
	baseBrandingEndpoint = "/branding"
	newBrandingEndpoint  = "/new"
	editBrandEndpoint    = "/edit" //TODO decide on how-to remove links
	getAllBrandEndpoints = "/get-all"
)

type newBrandingRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Links       map[string]string `json:"links"`
}

type getAllBrandResponse struct {
	Brands []ParsedBrands `json:"brands"`
}

type ParsedBrands struct {
	UserAddress string            `json:"brandAddress"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Links       map[string]string `json:"links,omitempty"`
}

type brandingHandler struct{}

func NewBrandingHandler(groupHandler *groupHandler) {
	h := brandingHandler{}

	//pub endpoiints
	pubEndpoints := []EndpointHandler{
		{Method: http.MethodGet, Path: getAllBrandEndpoints, HandlerFunc: h.getAllBrand},
	}
	pubEndpointGroupHandler := EndpointGroupHandler{
		Root:             baseBrandingEndpoint,
		Middleware:       []gin.HandlerFunc{},
		EndpointHandlers: pubEndpoints,
	}
	groupHandler.AddEndpointGroupHandler(pubEndpointGroupHandler)

	//auth endpooints
	authEndpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: newBrandingEndpoint, HandlerFunc: h.newBranding},
	}

	auth := middleware.Authorization(config.Config.Jwt.Secret)
	authEndpointGroupHandler := EndpointGroupHandler{
		Root:             baseBrandingEndpoint,
		Middleware:       []gin.HandlerFunc{auth},
		EndpointHandlers: authEndpoints,
	}
	groupHandler.AddEndpointGroupHandler(authEndpointGroupHandler)
}

func (h *brandingHandler) newBranding(c *gin.Context) {
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

	var req newBrandingRequest
	err = c.Bind(&req)
	if err != nil {
		err = errors.New("error while binding request: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	brand := model.Branding{
		UserAddress: userAddress,
		Name:        req.Name,
		Description: req.Description,
	}

	linksAsByte, err := json.Marshal(req.Links)
	if err != nil {
		err = errors.New("error while marshalling links: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	err = brand.SetLinks(string(linksAsByte))
	if err != nil {
		err = errors.New("error while setting brand links: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	err = storage.CreateBrand(&brand)
	if err != nil {
		err = errors.New("error while creating branding: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
}

func (h *brandingHandler) getAllBrand(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	brands, err := storage.GetAllBrands()
	if err != nil {
		err = errors.New("error while retrieving brands: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	var response getAllBrandResponse
	for _, b := range brands {
		links, err := b.GetLinks()
		if err != nil {
			err = errors.New("error while retrieving links for brand " + b.Name + " :" + err.Error())
			log.Error(err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
			return
		}
		p := ParsedBrands{
			UserAddress: b.UserAddress,
			Name:        b.Name,
			Description: b.Description,
			Links:       links,
		}
		response.Brands = append(response.Brands, p)
	}

	model.JsonResponse(c, http.StatusOK, response, nodeAddress, "")
}
