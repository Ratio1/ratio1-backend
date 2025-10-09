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
	editBrandEndpoint    = "/edit"
	getBrandsEndpoints   = "/get-brands"
)

type editBrandRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Links       map[string]string `json:"links"`
}

type getBrandsrequest struct {
	Addresses []string `json:"brandAddresses"`
}

type getBrandsResponse struct {
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
		{Method: http.MethodPost, Path: getBrandsEndpoints, HandlerFunc: h.getBrands},
	}
	pubEndpointGroupHandler := EndpointGroupHandler{
		Root:             baseBrandingEndpoint,
		Middleware:       []gin.HandlerFunc{},
		EndpointHandlers: pubEndpoints,
	}
	groupHandler.AddEndpointGroupHandler(pubEndpointGroupHandler)

	//auth endpooints
	authEndpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: editBrandEndpoint, HandlerFunc: h.editBrand},
	}

	auth := middleware.Authorization(config.Config.Jwt.Secret)
	authEndpointGroupHandler := EndpointGroupHandler{
		Root:             baseBrandingEndpoint,
		Middleware:       []gin.HandlerFunc{auth},
		EndpointHandlers: authEndpoints,
	}
	groupHandler.AddEndpointGroupHandler(authEndpointGroupHandler)
}

func (h *brandingHandler) editBrand(c *gin.Context) {
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

	var req editBrandRequest
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

	err = storage.SaveBrand(&brand)
	if err != nil {
		err = errors.New("error while saving brand: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
}

func (h *brandingHandler) getBrands(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	var req getBrandsrequest
	err = c.Bind(&req)
	if err != nil {
		err = errors.New("error while binding request: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	var response getBrandsResponse
	for _, a := range req.Addresses {
		b, err := storage.GetBrandByAddress(a)
		if err != nil {
			err = errors.New("error while retrieving brand: " + err.Error())
			log.Error(err.Error())
			model.JsonResponse(c, http.StatusInternalServerError, nil, nodeAddress, err.Error())
			return
		}
		if b == nil {
			continue
		}
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
