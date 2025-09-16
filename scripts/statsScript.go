package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/ethereum/go-ethereum/ethclient"
)

/*
MAKE sure to set all the needed variabels berfore running
*/

/*
func main() {
	launchTimeUTC := time.Date(2025, 5, 24, 16, 5, 0, 0, time.UTC)
	numberOfDays := int(time.Since(launchTimeUTC).Hours() / 24)
	BackfillStatsFrom(&launchTimeUTC, numberOfDays)
	DBConnect()
	GetFromBlockToBlockStats()
}*/

func BackfillStatsFrom(startTs *time.Time, days int) {
	ctx := context.Background()
	client, err := ethclient.Dial(InfuraApiUrl + InfuraSecret)
	if err != nil {
		fmt.Println("error while dialing client:", err)
		return
	}
	defer client.Close()

	latestBlock, err := getChainLastBlockNumber(client)
	if err != nil {
		fmt.Println("error getting last block number:", err)
		return
	}

	time.Sleep(SleepTime)
	cspAddresses, err := getAllCSPAddress(client)
	if err != nil {
		fmt.Println("Error while retrieving csp addresses:", err)
		return
	}

	var statstiche []model.Stats
	oldStats := &model.Stats{
		TotalTokenBurn:           big.NewInt(0),
		TotalNdContractTokenBurn: big.NewInt(0),
		TotalMinted:              big.NewInt(0),
		TotalPOAIRewards:         big.NewInt(0),
		LastBlockNumber:          0,
	}

	for i := 0; i < days; i++ {
		fmt.Println("Processing day", i+1, "of", days)
		dayEnd := startTs.Add(24 * time.Hour)
		fromBlock := oldStats.LastBlockNumber + 1
		time.Sleep(SleepTime)
		toBlock, err := findBlockByTimestampSmart(ctx, client, dayEnd, fromBlock, latestBlock)
		if err != nil {
			fmt.Println("error locating toBlock:", err)
			return
		}

		fmt.Println("  fromBlock:", fromBlock, " toBlock:", toBlock, " (day end:", dayEnd, ")")
		time.Sleep(SleepTime)
		allocEvents, err := fetchAllocationEvents(cspAddresses, fromBlock, toBlock, client)
		if err != nil {
			fmt.Println("Error fetching allocation events:", err)
			return
		}

		dailyPoaiReward := big.NewInt(0)
		for _, e := range allocEvents {
			dailyPoaiReward = dailyPoaiReward.Add(dailyPoaiReward, e.GetUsdcAmountPayed())
		}

		time.Sleep(SleepTime)
		dailyMinted, err := getPeriodMintedAmount(fromBlock, toBlock, client)
		if err != nil {
			fmt.Println("error getting daily minted:", err)
			return
		}

		time.Sleep(SleepTime)
		dailyTokenBurn, err := getPeriodBurnedAmount(fromBlock, toBlock, client)
		if err != nil {
			fmt.Println("error getting daily token burn:", err)
			return
		}

		time.Sleep(SleepTime)
		dailyNdContractTokenBurn, err := getPeriodNdContractBurnedAmount(fromBlock, toBlock, client)
		if err != nil {
			fmt.Println("error getting daily nd contract token burn:", err)
			return
		}

		time.Sleep(SleepTime)
		totalSupply, err := getTotalSupplyAt(ctx, client, toBlock)
		if err != nil {
			fmt.Println("error getting historical totalSupply:", err)
			return
		}

		time.Sleep(SleepTime)
		teamWalletsSupply, err := getTeamWalletsSupplyAt(ctx, client, toBlock)
		if err != nil {
			fmt.Println("error getting historical team wallets supply:", err)
			return
		}

		time.Sleep(SleepTime)
		dailyUsdcLocked, err := getUsdcLockedAt(ctx, client, cspAddresses, toBlock)
		if err != nil {
			fmt.Println("error getting historical USDC locked:", err)
			return
		}

		time.Sleep(SleepTime)
		dailyActiveJobs, err := getActiveJobsAt(ctx, client, toBlock)
		if err != nil {
			fmt.Println("error getting historical active jobs:", err)
			return
		}

		stats := model.Stats{
			CreationTimestamp:        dayEnd.UTC(),
			DailyActiveJobs:          dailyActiveJobs,
			DailyUsdcLocked:          dailyUsdcLocked,
			DailyTokenBurn:           dailyTokenBurn,
			DailyNdContractTokenBurn: dailyNdContractTokenBurn,
			DailyMinted:              dailyMinted,
			DailyPOAIRewards:         dailyPoaiReward,
			TotalSupply:              totalSupply,
			TeamWalletsSupply:        teamWalletsSupply,
			TotalTokenBurn:           big.NewInt(0).Add(oldStats.TotalTokenBurn, dailyTokenBurn),
			TotalNdContractTokenBurn: big.NewInt(0).Add(oldStats.TotalNdContractTokenBurn, dailyNdContractTokenBurn),
			TotalMinted:              big.NewInt(0).Add(oldStats.TotalMinted, dailyMinted),
			TotalPOAIRewards:         big.NewInt(0).Add(oldStats.TotalPOAIRewards, dailyPoaiReward),
			LastBlockNumber:          toBlock,
		}
		oldStats = &stats
		statstiche = append(statstiche, stats)
		startTs = &dayEnd
		time.Sleep(SleepTime)
	}

	//save on json file
	data, _ := json.MarshalIndent(statstiche, "", "  ")
	os.WriteFile("scripts/stats.json", data, 0644)
}

func GetFromBlockToBlockStats() {
	ctx := context.Background()
	client, err := ethclient.Dial(InfuraApiUrl + InfuraSecret)
	if err != nil {
		fmt.Println("error while dialing client:", err)
		return
	}
	defer client.Close()

	oldStats, err := getLatestStats()
	if err != nil {
		fmt.Println("error getting latest stats: " + err.Error())
		return
	} else if oldStats == nil {
		fmt.Println("empty latest stats")
		return
	}

	cspAddresses, err := getAllCSPAddress(client) // map[cspAddress]ownerAddress
	if err != nil {
		fmt.Println("Error while retrieving csp addresses: " + err.Error())
		return
	}

	fromBlock := oldStats.LastBlockNumber
	toBlock := int64(35581379 + 5) //allocation happened 35581379  HASH: 0x342a0b585de580424bcce9b72a4819483c5c61373d3b7f04f91d963c309d8b82, get five mor blocks

	allocEvents, err := fetchAllocationEvents(cspAddresses, fromBlock, toBlock, client)
	if err != nil {
		fmt.Println("Error fetching events: " + err.Error())
		return
	}

	dailyPoaiReward := big.NewInt(0)
	for _, e := range allocEvents {
		dailyPoaiReward = dailyPoaiReward.Add(dailyPoaiReward, e.GetUsdcAmountPayed())
	}

	time.Sleep(SleepTime)
	dailyMinted, err := getPeriodMintedAmount(fromBlock, toBlock, client)
	if err != nil {
		fmt.Println("error getting daily minted:", err)
		return
	}

	time.Sleep(SleepTime)
	dailyTokenBurn, err := getPeriodBurnedAmount(fromBlock, toBlock, client)
	if err != nil {
		fmt.Println("error getting daily token burn:", err)
		return
	}

	time.Sleep(SleepTime)
	dailyNdContractTokenBurn, err := getPeriodNdContractBurnedAmount(fromBlock, toBlock, client)
	if err != nil {
		fmt.Println("error getting daily nd contract token burn:", err)
		return
	}

	time.Sleep(SleepTime)
	totalSupply, err := getTotalSupplyAt(ctx, client, toBlock)
	if err != nil {
		fmt.Println("error getting historical totalSupply:", err)
		return
	}

	time.Sleep(SleepTime)
	teamWalletsSupply, err := getTeamWalletsSupplyAt(ctx, client, toBlock)
	if err != nil {
		fmt.Println("error getting historical team wallets supply:", err)
		return
	}

	time.Sleep(SleepTime)
	dailyUsdcLocked, err := getUsdcLockedAt(ctx, client, cspAddresses, toBlock)
	if err != nil {
		fmt.Println("error getting historical USDC locked:", err)
		return
	}

	time.Sleep(SleepTime)
	dailyActiveJobs, err := getActiveJobsAt(ctx, client, toBlock)
	if err != nil {
		fmt.Println("error getting historical active jobs:", err)
		return
	}

	stats := model.Stats{
		CreationTimestamp:        time.Date(2025, time.September, 15, 16, 02, 06, 0, time.UTC), //!HARDCODED
		DailyActiveJobs:          dailyActiveJobs,
		DailyUsdcLocked:          dailyUsdcLocked,
		DailyTokenBurn:           dailyTokenBurn,
		DailyNdContractTokenBurn: dailyNdContractTokenBurn,
		DailyMinted:              dailyMinted,
		DailyPOAIRewards:         dailyPoaiReward,
		TotalSupply:              totalSupply,
		TeamWalletsSupply:        teamWalletsSupply,
		TotalTokenBurn:           big.NewInt(0).Add(oldStats.TotalTokenBurn, dailyTokenBurn),
		TotalNdContractTokenBurn: big.NewInt(0).Add(oldStats.TotalNdContractTokenBurn, dailyNdContractTokenBurn),
		TotalMinted:              big.NewInt(0).Add(oldStats.TotalMinted, dailyMinted),
		TotalPOAIRewards:         big.NewInt(0).Add(oldStats.TotalPOAIRewards, dailyPoaiReward),
		LastBlockNumber:          toBlock,
	}

	data, _ := json.MarshalIndent(stats, "", "  ")
	os.WriteFile("stats.json", data, 0644)
}
