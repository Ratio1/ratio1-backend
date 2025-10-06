package service

import (
	"math/big"
	"os"
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
			LocalCurrency:     "EUR",
			ExchangeRatio:     0.85,
			BurnTimestamp:     time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
			TxHash:            "0xabc123",
		},
		{
			Id:                2,
			UsdcAmountSwapped: big.NewInt(2000000).String(),             // 2 USDC
			R1AmountBurned:    big.NewInt(1000000000000000000).String(), // 1 R1
			LocalCurrency:     "EUR",
			ExchangeRatio:     1.0,
			BurnTimestamp:     time.Date(2023, 10, 2, 15, 30, 0, 0, time.UTC),
			TxHash:            "0xdef456",
		},
	}

	csvBytes, err := GenerateBurnReportCSV(burnEvents)
	require.Nil(t, err)
	os.WriteFile("file.csv", csvBytes, 0644)
}
