package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/gin-gonic/gin"
)

const (
	baseBrandingEndpoint   = "/branding"
	editBrandEndpoint      = "/edit"
	editBrandLogoEndpoint  = "/edit-logo"
	getBrandsEndpoints     = "/get-brands"
	getBrandsLogosEndpoint = "/get-brand-logo"
	getPlatformsEndpoint   = "/get-platforms"
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
	Links       map[string]string `json:"links"`
}

type brandingHandler struct{}

func NewBrandingHandler(groupHandler *groupHandler) {
	h := brandingHandler{}

	//pub endpoiints
	pubEndpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: getBrandsEndpoints, HandlerFunc: h.getBrands},
		{Method: http.MethodGet, Path: getBrandsLogosEndpoint, HandlerFunc: h.getBrandLogo},
		{Method: http.MethodGet, Path: getPlatformsEndpoint, HandlerFunc: h.getPlatforms},
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
		{Method: http.MethodPost, Path: editBrandLogoEndpoint, HandlerFunc: h.editBrandLogo},
	}

	auth := middleware.Authorization(config.Config.Jwt.Secret)
	authEndpointGroupHandler := EndpointGroupHandler{
		Root:             baseBrandingEndpoint,
		Middleware:       []gin.HandlerFunc{auth, middleware.RequireApprovedKyc()},
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

	userAddress = strings.ToLower(userAddress)

	var req editBrandRequest
	err = c.Bind(&req)
	if err != nil {
		err = errors.New("error while binding request: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	brand, err := storage.GetBrandByAddress(userAddress)
	if err != nil {
		err = errors.New("error while retrieving brand: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	} else if brand == nil {
		brand = &model.Branding{
			UserAddress: userAddress,
		}
	}
	brand.Name = req.Name
	brand.Description = req.Description

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

	err = storage.SaveBrand(brand)
	if err != nil {
		err = errors.New("error while saving brand: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	model.JsonResponse(c, http.StatusOK, nil, nodeAddress, "")
}

func (h *brandingHandler) editBrandLogo(c *gin.Context) {
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
	userAddress = strings.ToLower(userAddress)

	// Bound the entire multipart request as well as the decoded image below.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxBrandLogoBytes+(1<<20))
	file, err := c.FormFile("logo")
	if err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			model.JsonResponse(c, http.StatusRequestEntityTooLarge, nil, nodeAddress, "logo request is too large")
			return
		}
		err = errors.New("error while retrieving logo: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	//get file reader
	fileReader, err := file.Open()
	if err != nil {
		err = errors.New("error while opening logo: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}
	defer fileReader.Close()

	logo, contentType, err := service.NormalizeBrandLogo(fileReader)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrBrandLogoTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		model.JsonResponse(c, status, nil, nodeAddress, err.Error())
		return
	}
	filename := "logo.png"
	if contentType == "image/jpeg" {
		filename = "logo.jpg"
	}

	brand, err := storage.GetBrandByAddress(userAddress)
	if err != nil {
		err = errors.New("error while retrieving brand: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	} else if brand == nil {
		brand = &model.Branding{
			UserAddress: userAddress,
			Links:       "{}",
		}
	}

	err = brand.SetLogoBase64(bytes.NewReader(logo), filename)
	if err != nil {
		err = errors.New("error while setting brand logo: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	err = storage.SaveBrand(brand)
	if err != nil {
		err = errors.New("error while saving brand: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
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
		a = strings.ToLower(a)
		b, err := storage.GetBrandByAddress(a)
		if err != nil {
			err = errors.New("error while retrieving brand: " + err.Error())
			log.Error(err.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
			return
		}
		if b == nil {
			continue
		}
		links, err := b.GetLinks()
		if err != nil {
			err = errors.New("error while retrieving links for brand " + b.Name + " :" + err.Error())
			log.Error(err.Error())
			model.JsonResponse(c, http.StatusBadRequest, nil, "", err.Error())
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

func (h *brandingHandler) getBrandLogo(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}

	address := c.Query("address")
	if address == "" {
		err = errors.New("no address provided")
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}
	address = strings.ToLower(address)

	brand, err := storage.GetBrandByAddress(address)
	if err != nil {
		err = errors.New("error while retrieving brand: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}
	if brand == nil {
		err = errors.New("brand does not exist")
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	logo, err := brand.GetLogoBase64()
	if err != nil {
		err = errors.New("error while retrieving logo from r1fs: " + err.Error())
		log.Error(err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	// Validate legacy stored uploads too; never serve their sniffed content type.
	logo, contentType, err := service.NormalizeBrandLogo(bytes.NewReader(logo))
	if err != nil {
		model.JsonResponse(c, http.StatusNotFound, nil, nodeAddress, "logo is unavailable")
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, contentType, logo)
}

func (h *brandingHandler) getPlatforms(c *gin.Context) {
	nodeAddress, err := service.GetAddress()
	if err != nil {
		log.Error("error while retrieving node address: " + err.Error())
		model.JsonResponse(c, http.StatusInternalServerError, nil, "", err.Error())
		return
	}
	response := model.Platform(0).GetPlatforms()
	model.JsonResponse(c, http.StatusOK, response, nodeAddress, "")
}
