package main

import "math/big"

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
	allDailyPoaiRewards := make(map[int64]*big.Int)
	for _, alloc := range allAllocations {
		amount := alloc.GetUsdcAmountPayed()
		if _, ok := allDailyPoaiRewards[alloc.BlockNumber]; !ok {
			allDailyPoaiRewards[alloc.BlockNumber] = big.NewInt(0)
		}
		allDailyPoaiRewards[alloc.BlockNumber].Add(allDailyPoaiRewards[alloc.BlockNumber], amount)
	}
	//update all stats with daily poai rewards and total poai rewards
	for k, v := range allDailyPoaiRewards {
		for i, stat := range allStats {
			if stat.LastBlockNumber >= k {
				stat.DailyPOAIRewards = v
				if stat.TotalPOAIRewards == nil {
					stat.TotalPOAIRewards = big.NewInt(0)
				}
				stat.TotalPOAIRewards.Add(allStats[i-1].TotalPOAIRewards, v)
				allStats[i] = stat
			}
		}
	}

	// recalculate total daily usdc locked
	for i, stat := range allStats {
		allStats[i].DailyUsdcLocked = big.NewInt(0).Sub(stat.DailyUsdcLocked, stat.TotalPOAIRewards)
	}

	//save on storage
	for _, stat := range allStats {
		err = updateStats(&stat)
		if err != nil {
			panic(err)
		}
	}
}
