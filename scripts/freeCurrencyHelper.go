package main

import (
	"errors"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
)

func getFreeCurrencyValues() (map[string]float64, error) {
	response := struct {
		Data map[string]float64 `json:"data"`
	}{}
	err := process.HttpGet("https://api.freecurrencyapi.com/v1/latest?apikey="+FreeCurrencyApiKey, &response)
	if err != nil {
		return nil, errors.New("error while making request: " + err.Error())
	}
	return response.Data, nil
}
