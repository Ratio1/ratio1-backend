package handlers

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/proxy/middleware"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/service"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"

	"github.com/gin-gonic/gin"
)

const (
	adminBaseEndpoint  = "/admin"
	newsLetterEndpoint = "/news"
)

type adminHandler struct{}

func NewAdminHandler(groupHandler *groupHandler) {
	h := &adminHandler{}

	endpoints := []EndpointHandler{
		{Method: http.MethodPost, Path: newsLetterEndpoint, HandlerFunc: h.sendNewsLetterEmail},
	}

	endpointGroupHandler := EndpointGroupHandler{
		Root:             adminBaseEndpoint,
		Middleware:       []gin.HandlerFunc{middleware.Authorization(config.Config.Jwt.Secret)},
		EndpointHandlers: endpoints,
	}

	groupHandler.AddEndpointGroupHandler(endpointGroupHandler)
}

func (h *adminHandler) sendNewsLetterEmail(c *gin.Context) {
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

	fileHeader, err := c.FormFile("news")
	if err != nil {
		log.Error("error while retrieving file from post: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}

	subject := c.PostForm("subject")
	if subject == "" {
		log.Error("error while retrieving subject: subject is empty")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "error while retrieving subject: subject is empty")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		log.Error("error while opening file: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}
	defer file.Close()

	contentBytes, err := io.ReadAll(file)
	if err != nil {
		log.Error("error while reading the file: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}
	htmlContent := string(contentBytes)

	emails, err := storage.GetAllUsersEmails()
	if err != nil {
		log.Error("error while retrieving all users emails: " + err.Error())
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, err.Error())
		return
	}
	if len(emails) == 0 {
		log.Error("error while retrieving all users emails: lenght is 0")
		model.JsonResponse(c, http.StatusBadRequest, nil, nodeAddress, "error while retrieving all users emails: lenght is 0")
		return
	}

	errCh := make(chan error, (len(emails)/500)+1)
	var wg sync.WaitGroup
	for i := 0; i < len(emails); i += 500 {
		end := i + 500
		end = min(end, len(emails))
		wg.Add(1)
		go func(_email []string) {
			defer wg.Done()
			if err := service.SendNewsEmail(_email, subject, htmlContent); err != nil {
				errCh <- errors.New("error while sending email to user: " + strings.Join(_email, " | ") + " with error: " + err.Error())
				return
			}
		}(emails[i:end])

	}
	wg.Wait()
	close(errCh)

	if len(errCh) > 0 {
		var errorMsgs []string
		for err := range errCh {
			errorMsgs = append(errorMsgs, err.Error())
		}
		model.JsonResponse(c, http.StatusInternalServerError, emails, nodeAddress, strings.Join(errorMsgs, " | "))
		return
	}
	model.JsonResponse(c, http.StatusOK, emails, nodeAddress, "") //TODO remove emails from answers

}
