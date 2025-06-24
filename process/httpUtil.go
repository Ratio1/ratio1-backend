package process

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/GoRoadster/go-log"
)

const (
	contentTypeKey   = "Content-Type"
	contentTypeValue = "application/json"

	binancePriceUrlEGLD = "https://api.binance.com/api/v3/ticker/price?symbol=EGLDUSDT"
)

type HttpHeaderPair struct {
	Key   string
	Value string
}

type binanceResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

func GetEgldPrice() (float64, error) {
	var br binanceResponse
	err := HttpGet(binancePriceUrlEGLD, &br)
	if err != nil {
		return -1, err
	}
	if br.Price == "" {
		return -1, errors.New("failed to fetch EGLD price")
	}
	vFloat, err := strconv.ParseFloat(br.Price, 64)
	if err != nil {
		return -1, err
	}

	return vFloat, nil
}

func HttpGet(url string, castTarget interface{}, headers ...HttpHeaderPair) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if headers != nil {
		for _, head := range headers {
			req.Header.Set(head.Key, head.Value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		bodyCloseErr := resp.Body.Close()
		if bodyCloseErr != nil {
			log.Warn("HttpGet - error while trying to close response body", "err", bodyCloseErr.Error())
		}
	}()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(respBytes, castTarget)
}

func HttpPost(url string, payload interface{}, response interface{}, headers ...HttpHeaderPair) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set(contentTypeKey, contentTypeValue)
	if headers != nil {
		for _, head := range headers {
			req.Header.Set(head.Key, head.Value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("error while doing http post request: " + err.Error())
	}
	defer func() {
		bodyCloseErr := resp.Body.Close()
		if bodyCloseErr != nil {
			log.Warn("HttpPost - error while trying to close response body", "err", bodyCloseErr.Error())
		}
	}()
	resBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(resBody, &response)
}

func HttpPostWithUrlEncoded(url string, payload interface{}, response interface{}, headers ...HttpHeaderPair) error {
	var reqBody io.Reader

	switch v := payload.(type) {
	case string:
		reqBody = strings.NewReader(v)
	default:
		return errors.New("unsupported payload type")
	}

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, url, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, head := range headers {
		req.Header.Set(head.Key, head.Value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("unexpected status code: " + resp.Status)
	}
	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(resBody, &response)
}
