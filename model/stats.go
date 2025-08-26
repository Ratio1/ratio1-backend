package model

import (
	"math/big"
	"time"
)

type Stats struct {
	CreationTimestamp        time.Time `gorm:"primarykey;unique" json:"creationTimestamp"` //primary key
	DailyActiveJobs          int       `gorm:"type:integer;default:null" json:"dailyActiveJobs"`
	DailyUsdcLocked          *big.Int  `gorm:"type:numeric;default:null" json:"dailyUsdcLocked"`
	DailyTokenBurn           *big.Int  `gorm:"type:numeric;default:null" json:"dailyTokenBurn"`
	TotalTokenBurn           *big.Int  `gorm:"type:numeric;default:null" json:"totalTokenBurn"`
	DailyNdContractTokenBurn *big.Int  `gorm:"type:numeric;default:null" json:"dailyNdContractTokenBurn"`
	TotalNdContractTokenBurn *big.Int  `gorm:"type:numeric;default:null" json:"totalNdContractTokenBurn"`
	DailyPOAIRewards         *big.Int  `gorm:"type:numeric;default:null" json:"dailyPOAIRewards"`
	TotalPOAIRewards         *big.Int  `gorm:"type:numeric;default:null" json:"totalPOAIRewards"`
	DailyMinted              *big.Int  `gorm:"type:numeric;default:null" json:"dailyMinted"`
	TotalMinted              *big.Int  `gorm:"type:numeric;default:null" json:"totalMinted"`
	TotalSupply              *big.Int  `gorm:"type:numeric;default:null" json:"totalSupply"`
	TeamWalletsSupply        *big.Int  `gorm:"type:numeric;default:null" json:"teamWalletsSupply"`
	LastBlockNumber          int64     `gorm:"type:bigint;default:null" json:"lastBlockNumber"`
}
