package service

import (
	"encoding/csv"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
)

func GenerateBurnReportCSV(burnEvents []model.BurnEvent) ([]byte, error) {
	var csvData strings.Builder
	writer := csv.NewWriter(&csvData)

	// Write CSV header
	header := []string{"USDC swapped", "", "R1 burned", "", "Local Currrency", "", "Burn Timestamp", "Transaction Hash"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write CSV records
	totalR1Burned := big.NewInt(0)
	totalUsdcSwapped := big.NewInt(0)
	totalPreferredCurrency := float64(0)
	for _, event := range burnEvents {
		totalR1Burned.Add(totalR1Burned, event.GetR1AmountBurned())
		totalUsdcSwapped.Add(totalUsdcSwapped, event.GetUsdcAmountSwapped())
		totalPreferredCurrency += GetAmountAsFloat(event.GetUsdcAmountSwapped(), model.UsdcDecimals) * event.ExchangeRatio
		record := []string{
			fmt.Sprintf("%.2f", GetAmountAsFloat(event.GetUsdcAmountSwapped(), model.UsdcDecimals)),
			"USDC",
			fmt.Sprintf("%.2f", GetAmountAsFloat(event.GetR1AmountBurned(), model.R1Decimals)),
			"R1",
			fmt.Sprintf("%.2f", GetAmountAsFloat(event.GetUsdcAmountSwapped(), model.UsdcDecimals)*event.ExchangeRatio),
			event.LocalCurrency,
			event.BurnTimestamp.Format(time.RFC3339),
			event.TxHash,
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	//add empty line
	if err := writer.Write([]string{}); err != nil {
		return nil, err
	}
	// Write totals
	totalRecord := []string{
		"Total USDC swapped:",
		fmt.Sprintf("%.2f", GetAmountAsFloat(totalUsdcSwapped, model.UsdcDecimals)),
		"",
		"Total R1 burned:",
		fmt.Sprintf("%.2f", GetAmountAsFloat(totalR1Burned, model.R1Decimals)),
		"",
		"Total Local Currency:",
		fmt.Sprintf("%.2f", totalPreferredCurrency),
	}
	if err := writer.Write(totalRecord); err != nil {
		return nil, err
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return []byte(csvData.String()), nil
}
