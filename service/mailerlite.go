package service

import (
	"fmt"
	"net/http"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
)

type AddSubscriberRequest struct {
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
}

func AddSubscriber(email string) error {
	if config.Config.Api.DevTesting {
		return nil
	}
	url := fmt.Sprintf("%s/subscribers", config.Config.MailerLite.Url)

	request := AddSubscriberRequest{
		Email:  email,
		Groups: []string{config.Config.MailerLite.GroupId},
	}
	headers := []process.HttpHeaderPair{
		{
			Key:   "Authorization",
			Value: "Bearer " + config.Config.MailerLite.ApiKey,
		},
	}

	var response interface{}
	return process.HttpPost(url, request, &response, headers...)
}

func RemoveSubscriber(email string) error {
	if config.Config.Api.DevTesting {
		return nil
	}
	url := fmt.Sprintf("%s/subscribers/%s", config.Config.MailerLite.Url, email)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+config.Config.MailerLite.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("unexpected response code %d", resp.StatusCode)
	}

	return nil
}
