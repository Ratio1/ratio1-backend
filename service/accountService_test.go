package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/stretchr/testify/require"
)

func Test_TrimWhitespacesAndToLower(t *testing.T) {
	require.Equal(t, "alessandro@bh.network", TrimWhitespacesAndToLower("  alessandro@bh.network  "))
}

type ResponseAccount struct {
	Data struct {
		Email          string `json:"email"`
		EmailConfirmed bool   `json:"emailConfirmed"`
		PendingEmail   string `json:"pendingEmail"`
		Address        string `json:"address"`
		BscAddress     string `json:"bscAddress"`
		KycRef         string `json:"kycRef"`
		KycStatus      string `json:"kycStatus"`
		IsBlacklisted  bool   `json:"isBlacklisted"`
		ReceiveUpdates bool   `json:"receiveUpdates"`
		Tier           int    `json:"tier"`
		MaxTickets     int    `json:"maxTickets"`
	} `json:"data"`
	Error string `json:"error"`
}

func TestSpam(t *testing.T) {
	numRetries := 10000

	responses := make([]string, numRetries)

	for i := 0; i < numRetries; i++ {
		go func(index int) {
			getAccount(index, &responses)
		}(i)
	}

	time.Sleep(3 * time.Second)

	for _, resp := range responses {
		fmt.Println(resp)
	}
}

func getAccount(index int, responses *[]string) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBZGRyZXNzIjoiMHgxMjE3NmNFNzY0MDE1Yjk4MzA2MUE0NjdCMWJFRUFmNDFGYWQ5NDJBIiwiaXNzIjoibG9jYWxob3N0OjUwMDAiLCJleHAiOjE3MTgyNjQxNzB9.N1TRnYoxhfAePmtlsJhhUO3DqrJLSgvBX_rgYQp9Tt4"
	headers := []process.HttpHeaderPair{
		{
			Key:   "Authorization",
			Value: "Bearer " + token,
		},
	}

	var resp ResponseAccount
	url := "http://127.0.0.1:5000/accounts/account"
	err := process.HttpGet(url, &resp, headers...)
	if err != nil {
		(*responses)[index] = fmt.Sprintf("go %d err: %v", index, err)
		return
	}

	if resp.Error != "" {
		(*responses)[index] = fmt.Sprintf("go %d resp err: %v", index, resp.Error)
		return
	}

	if resp.Error == "" {
		(*responses)[index] = fmt.Sprintf("go %d success: %v %v", index, resp.Data.Address, resp.Data.KycStatus)
		return
	}

	if resp.Data.Address == "" {
		(*responses)[index] = fmt.Sprintf("go %d empty address: %v", index, resp.Error)
		return
	}
}
