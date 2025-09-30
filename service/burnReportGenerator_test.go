package service

import (
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/stretchr/testify/require"
)

func Test_BurnReportgenerator(t *testing.T) {
	// Mock data
	burnEvents := []model.BurnEvent{
		{
			Id:                1,
			UsdcAmountSwapped: big.NewInt(1000000).String(),            // 1 USDC
			R1AmountBurned:    big.NewInt(500000000000000000).String(), // 0.5 R1
			PreferredCurrency: "EUR",
			ExchangeRatio:     0.85,
			BurnTimestamp:     time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
			TxHash:            "0xabc123",
		},
		{
			Id:                2,
			UsdcAmountSwapped: big.NewInt(2000000).String(),             // 2 USDC
			R1AmountBurned:    big.NewInt(1000000000000000000).String(), // 1 R1
			PreferredCurrency: "EUR",
			ExchangeRatio:     1.0,
			BurnTimestamp:     time.Date(2023, 10, 2, 15, 30, 0, 0, time.UTC),
			TxHash:            "0xdef456",
		},
	}

	csvBytes, err := GenerateBurnReportCSV(burnEvents)
	require.Nil(t, err)

	csvContent := string(csvBytes)
	expectedHeaders := "USDC swapped,,R1 burned,,Local Currrency,,Burn Timestamp,Transaction Hash"
	require.True(t, strings.Contains(csvContent, expectedHeaders))
	require.True(t, strings.Contains(csvContent, "1.00,USDC,0.50,R1,0.85,EUR,2023-10-01T12:00:00Z,0xabc123"))
	require.True(t, strings.Contains(csvContent, "2.00,USDC,1.00,R1,2.00,EUR,2023-10-02T15:30:00Z,0xdef456"))
	require.True(t, strings.Contains(csvContent, "Total USDC swapped:,3.00,,Total R1 burned:,1.50,,Total Local Currency:,2.85"))

	os.WriteFile("file.csv", csvBytes, 0644)
}
