package storage

import (
	"errors"
	"math/big"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
)

func CreateStats(stats *model.Stats) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	row := map[string]any{
		"creation_timestamp":           stats.CreationTimestamp,
		"daily_active_jobs":            stats.DailyActiveJobs,
		"daily_usdc_locked":            toNumericExpr(stats.DailyUsdcLocked),
		"daily_token_burn":             toNumericExpr(stats.DailyTokenBurn),
		"total_token_burn":             toNumericExpr(stats.TotalTokenBurn),
		"daily_nd_contract_token_burn": toNumericExpr(stats.DailyNdContractTokenBurn),
		"total_nd_contract_token_burn": toNumericExpr(stats.TotalNdContractTokenBurn),
		"daily_poai_rewards":           toNumericExpr(stats.DailyPOAIRewards),
		"total_poai_rewards":           toNumericExpr(stats.TotalPOAIRewards),
		"daily_minted":                 toNumericExpr(stats.DailyMinted),
		"total_minted":                 toNumericExpr(stats.TotalMinted),
		"total_supply":                 toNumericExpr(stats.TotalSupply),
		"team_wallets_supply":          toNumericExpr(stats.TeamWalletsSupply),
		"last_block_number":            stats.LastBlockNumber, // bigint
	}

	res := db.Table("stats").Create(row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("no row inserted")
	}
	return nil
}

func GetLatestStats() (*model.Stats, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var stats model.Stats
	tx := db.Order("creation_timestamp DESC").First(&stats)

	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}

	return &stats, nil
}

func GetAllStatsASC() (*[]model.Stats, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var stats []model.Stats
	tx := db.Order("creation_timestamp ASC").Find(&stats)

	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}

	return &stats, nil
}

func toNumericExpr(x *big.Int) any {
	if x == nil {
		return nil // scrive NULL
	}
	return gorm.Expr("?::numeric", x.String())
}
