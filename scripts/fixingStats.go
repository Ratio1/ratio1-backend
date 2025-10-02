package main

import (
	"math/big"
	"time"
)

func main() {
	DBConnect()
	allStats, err := getStatsAfterBlockASC(0)
	if err != nil {
		panic(err)
	}

	allAllocations, err := getAllAllocations()
	if err != nil {
		panic(err)
	}

	//get all dayly poai rewards ordered by block number
	allDailyPoaiRewards := make(map[int]*big.Int)
	for _, alloc := range allAllocations {
		amount := alloc.GetUsdcAmountPayed()
		if _, ok := allDailyPoaiRewards[getEpoch(alloc.AllocationCreation)]; !ok {
			allDailyPoaiRewards[getEpoch(alloc.AllocationCreation)] = big.NewInt(0)
		}
		allDailyPoaiRewards[getEpoch(alloc.AllocationCreation)].Add(allDailyPoaiRewards[getEpoch(alloc.AllocationCreation)], amount)
	}

	//update all stats with daily poai rewards and total poai rewards
	for k, v := range allDailyPoaiRewards {
		for i, stat := range allStats {
			if getEpoch(stat.CreationTimestamp) == k {
				stat.DailyPOAIRewards = v
				if stat.TotalPOAIRewards == nil {
					stat.TotalPOAIRewards = big.NewInt(0)
				}
				stat.TotalPOAIRewards.Add(allStats[i-1].TotalPOAIRewards, v)
				allStats[i] = stat
				break
			}
		}
	}

	// recalculate total daily usdc locked
	for i, stat := range allStats {
		allStats[i].DailyUsdcLocked.Sub(stat.DailyUsdcLocked, stat.TotalPOAIRewards)
	}

	//save on storage
	for _, stat := range allStats {
		err = updateStats(&stat)
		if err != nil {
			panic(err)
		}
	}
}

func getEpoch(date time.Time) int {
	mainnetStart := time.Unix(1748016000, 0)
	return int(date.Sub(mainnetStart) / (24 * time.Hour))
}
