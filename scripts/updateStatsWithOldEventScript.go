package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
)

/*
MAKE sure to set all the needed variabels berfore running
*/
/*
func main() {
	DBConnect()

	UpdateStatsFromFile() //if you have to change some data on the json file before saving it
	UpdateOldStats()
}*/

func UpdateOldStats() {
	//retrieve old allocations plus all stats
	data, err := os.ReadFile("old.json")
	if err != nil {
		fmt.Println("error reading file: ", err.Error())
		return
	}

	var oldAlloc []model.Allocation
	err = json.Unmarshal(data, &oldAlloc)
	if err != nil {
		fmt.Println("error while unmarshal json data to allocation: ", err.Error())
	}

	//calculate min to remove uninterested stats
	min := int64(math.MaxInt)
	for _, alloc := range oldAlloc {
		if alloc.BlockNumber < min {
			min = alloc.BlockNumber
		}
	}

	stats, err := getStatsAfterBlockASC(min)
	if err != nil {
		fmt.Println("error while retreaving stats from db: ", err.Error())
	}

	//change stats
	changeStats := make(map[int]any)
	for _, alloc := range oldAlloc {
		for i, stat := range stats {
			if alloc.BlockNumber <= stat.LastBlockNumber {
				stats[i].DailyPOAIRewards = big.NewInt(0).Add(stat.DailyPOAIRewards, alloc.GetUsdcAmountPayed())
				changeStats[i] = true
				break
			}
		}
	}

	//update total poai rewards
	value := big.NewInt(0)
	for i, stat := range stats {
		value = big.NewInt(0).Add(value, stat.DailyPOAIRewards)
		stats[i].TotalPOAIRewards = value
	}

	olddata, _ := json.Marshal(stats)
	_ = os.WriteFile("changeStats.json", olddata, 0644)

	for _, stat := range stats {
		updateStats(&stat)
	}

}

func UpdateStatsFromFile() {
	data, err := os.ReadFile("changeStats.json")
	if err != nil {
		fmt.Println("error reading file: ", err.Error())
		return
	}

	var stats []model.Stats
	err = json.Unmarshal(data, &stats)
	if err != nil {
		fmt.Println("error while unmarshal json data to allocation: ", err.Error())
	}

	for _, stat := range stats {
		updateStats(&stat)
	}
}
