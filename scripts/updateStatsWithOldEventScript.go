package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
)

/*
MAKE sure to set all the needed variabels berfore running
*/

/*
func main() {
	updateOldStats()
}*/

func updateOldStats() {
	//retrieve old allocations plus all stats
	data, err := os.ReadFile("old.json")
	if err != nil {
		fmt.Println("error reading file: ", err.Error())
	}

	var oldAlloc []model.Allocation
	err = json.Unmarshal(data, &oldAlloc)
	if err != nil {
		fmt.Println("error while unmarshal json data to allocation: ", err.Error())
	}

	stats, err := storage.GetAllStatsASC()
	if err != nil {
		fmt.Println("error while retreaving stats from db: ", err.Error())
	}

	//calculate min & max to remove uninterested stats
	max := int64(math.MinInt)
	min := int64(math.MaxInt)
	for _, alloc := range oldAlloc {
		if alloc.BlockNumber > max {
			max = alloc.BlockNumber
		} else if alloc.BlockNumber < min {
			min = alloc.BlockNumber
		}
	}

	var interestedStats []model.Stats
	prevIsBigger := false
	for _, stat := range *stats {
		if stat.LastBlockNumber >= max && prevIsBigger {
			break
		}
		if stat.LastBlockNumber >= max {
			prevIsBigger = true
		}
		if stat.LastBlockNumber >= min {
			interestedStats = append(interestedStats, stat)
		}
	}

	//change stats
	changeStats := make(map[int]any)
	for _, alloc := range oldAlloc {
		for i, stat := range interestedStats {
			if alloc.BlockNumber <= stat.LastBlockNumber {
				interestedStats[i].DailyPOAIRewards = big.NewInt(0).Add(stat.DailyPOAIRewards, alloc.GetUsdcAmountPayed())
				changeStats[i] = true
				break
			}
		}
	}

	for k := range changeStats {
		storage.UpdateStats(&interestedStats[k])
	}
}
