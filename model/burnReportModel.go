package model

import (
	"math/big"
	"time"
)

type BurnEvent struct {
	Id            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	BurnTimestamp time.Time `json:"burnTimestamp"`
	BlockNumber   int64     `gorm:"type:bigint;not null" json:"blockNumber"`
	TxHash        string    `gorm:"type:varchar(66);not null" json:"txHash"`

	CspAddress        string  `gorm:"type:varchar(66);not null;index" json:"cspAddress"`
	CspOwner          string  `gorm:"type:varchar(66);not null" json:"cspOwner"`
	UsdcAmountSwapped string  `gorm:"type:numeric;default:null" json:"usdcAmountSwapped"`
	R1AmountBurned    string  `gorm:"type:numeric;default:null" json:"r1AmountBurned"`
	LocalCurrency     string  `gorm:"type:varchar(3);" json:"localCurrency"`
	ExchangeRatio     float64 `gorm:"type:numeric" json:"exchangeRatio"`

	CspProfile UserInfo `gorm:"foreignKey:CspOwner;references:BlockchainAddress" json:"cspProfile"`
}

func (b *BurnEvent) GetUsdcAmountSwapped() *big.Int {
	amount, ok := new(big.Int).SetString(b.UsdcAmountSwapped, 10)
	if !ok {
		return big.NewInt(0)
	}
	return amount
}

func (b *BurnEvent) SetUsdcAmountSwapped(amount *big.Int) {
	if amount == nil {
		b.UsdcAmountSwapped = "0"
	} else {
		b.UsdcAmountSwapped = amount.String()
	}
}

func (b *BurnEvent) GetR1AmountBurned() *big.Int {
	amount, ok := new(big.Int).SetString(b.R1AmountBurned, 10)
	if !ok {
		return big.NewInt(0)
	}
	return amount
}

func (b *BurnEvent) SetR1AmountBurned(amount *big.Int) {
	if amount == nil {
		b.R1AmountBurned = "0"
	} else {
		b.R1AmountBurned = amount.String()
	}
}
