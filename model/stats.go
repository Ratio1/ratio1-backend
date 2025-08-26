package model

import (
	"math/big"
	"time"
)

type Stats struct {
	Day                      time.Time `gorm:"primarykey;unique" json:"day"` //primary key
	DailyActiveJobs          int       `json:"dailyActiveJobs"`
	DailyUsdcLocked          *big.Int  `json:"dailyUsdcLocked"`
	DailyTokenBurn           *big.Int  `json:"dailyTokenBurn"`
	TotalTokenBurn           *big.Int  `json:"totalTokenBurn"`
	DailyNdContractTokenBurn *big.Int  `json:"dailyNdContractTokenBurn"`
	TotalNdContractTokenBurn *big.Int  `json:"totalNdContractTokenBurn"`
	DailyPOAIRewards         *big.Int  `json:"dailyPOAIRewards"`
	TotalPOAIRewards         *big.Int  `json:"totalPOAIRewards"`
	DailyMinted              *big.Int  `json:"dailyMinted"`
	TotalMinted              *big.Int  `json:"totalMinted"`
	TotalSupply              *big.Int  `json:"totalSupply"`
	TeamWalletsSupply        *big.Int  `json:"teamWalletsSupply"`
	LastBlockNumber          int64     `json:"lastBlockNumber"`
}
