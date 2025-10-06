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
	writer.Comma = ';'

	// Write CSV header
	header := []string{"USDC swapped", "", "R1 burned", "", "Local Currrency", "", "Burn Timestamp(UTC)", "Transaction Hash"}
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
			fmt.Sprintf("%.6f", GetAmountAsFloat(event.GetUsdcAmountSwapped(), model.UsdcDecimals)),
			"USDC",
			fmt.Sprintf("%.6f", GetAmountAsFloat(event.GetR1AmountBurned(), model.R1Decimals)),
			"R1",
			fmt.Sprintf("%.6f", GetAmountAsFloat(event.GetUsdcAmountSwapped(), model.UsdcDecimals)*event.ExchangeRatio),
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
	}
	if err := writer.Write(totalRecord); err != nil {
		return nil, err
	}

	totalRecord = []string{
		"Total R1 burned:",
		fmt.Sprintf("%.2f", GetAmountAsFloat(totalR1Burned, model.R1Decimals)),
	}
	if err := writer.Write(totalRecord); err != nil {
		return nil, err
	}

	//add empty line
	if err := writer.Write([]string{}); err != nil {
		return nil, err
	}

	disclaimerLines := strings.Split(DislaimerText, "\n")
	for _, line := range disclaimerLines {
		if err := writer.Write([]string{line}); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return []byte(csvData.String()), nil
}

const DislaimerText = `Protocol Burn Fee.
The 'burn' recorded in this report represents a protocol-level fee required to execute computation on Ratio1 Edge Nodes.
The burned tokens are irrevocably destroyed on-chain and permanently removed from circulation.
This amount is not a payment to Ratio1, its affiliates, or any Node Provider, and is economically analogous to blockchain network (gas) fees.
When accompanied by this report, the burn may be treated by the CSP as a documented network fee for internal accounting purposes (not tax advice).
For definitions, responsibilities, and limitations of liability, please refer to the Ratio1 Terms & Conditions:
https://ratio1.ai/terms-and-conditions`
