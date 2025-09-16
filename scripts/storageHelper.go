package main

import (
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DatabaseConfig struct {
	User         string
	Password     string
	Host         string
	Port         int
	DbName       string
	MaxOpenConns int
	MaxIdleConns int
	SslMode      string
}

func DBConnect() {
	once.Do(func() {
		fmt.Println(sql.Drivers())
		sqlDb, err := sql.Open("postgres", Database.Url())
		if err != nil {
			panic(err)
		}
		sqlDb.SetMaxOpenConns(Database.MaxOpenConns)
		sqlDb.SetMaxIdleConns(Database.MaxIdleConns)
		conn, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDb}))
		if err != nil {
			panic(err)
		}

		database = conn
	})
}

func (d DatabaseConfig) Url() string {
	format := "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s"
	return fmt.Sprintf(format, d.Host, d.Port, d.User, d.Password, d.DbName, d.SslMode)
}

func GetDB() (*gorm.DB, error) {
	if database == nil {
		return nil, NoDBError
	}

	return database, nil
}

func getAllActiveKyc() ([]*model.Kyc, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var acc []*model.Kyc
	txRead := db.Find(&acc, "kyc_status = ? AND is_active = ? AND has_been_deleted = ?", "approved", "true", "false")
	if txRead.Error != nil {
		return nil, txRead.Error
	}

	return acc, nil
}

func getAccountByEmail(email string) (*model.Account, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var acc model.Account
	txRead := db.Find(&acc, "email = ?", email)
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, false, nil
	}

	return &acc, true, nil
}

type statsRow struct {
	CreationTimestamp        time.Time
	DailyActiveJobs          int
	DailyUsdcLocked          *string
	DailyTokenBurn           *string
	TotalTokenBurn           *string
	DailyNdContractTokenBurn *string
	TotalNdContractTokenBurn *string
	DailyPOAIRewards         *string
	TotalPOAIRewards         *string
	DailyMinted              *string
	TotalMinted              *string
	TotalSupply              *string
	TeamWalletsSupply        *string
	LastBlockNumber          int64
}

func getStatsAfterBlockASC(blockNumber int64) ([]model.Stats, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var rows []statsRow
	err = db.Model(&model.Stats{}).
		Select(`
			creation_timestamp,
			daily_active_jobs,
			daily_usdc_locked::text          AS daily_usdc_locked,
			daily_token_burn::text           AS daily_token_burn,
			total_token_burn::text           AS total_token_burn,
			daily_nd_contract_token_burn::text AS daily_nd_contract_token_burn,
			total_nd_contract_token_burn::text AS total_nd_contract_token_burn,
			daily_poai_rewards::text         AS daily_poai_rewards,
			total_poai_rewards::text         AS total_poai_rewards,
			daily_minted::text               AS daily_minted,
			total_minted::text               AS total_minted,
			total_supply::text               AS total_supply,
			team_wallets_supply::text        AS team_wallets_supply,
			last_block_number
		`).
		Where("last_block_number >= ?", blockNumber).
		Order("creation_timestamp ASC").
		Scan(&rows).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	out := make([]model.Stats, 0, len(rows))
	for i := range rows {
		out = append(out, *rowToModel(&rows[i]))
	}
	return out, nil
}

func getLatestStats() (*model.Stats, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var r statsRow
	// selezioniamo castando i NUMERIC a testo
	err = db.Model(&model.Stats{}).
		Select(`
			creation_timestamp,
			daily_active_jobs,
			daily_usdc_locked::text          AS daily_usdc_locked,
			daily_token_burn::text           AS daily_token_burn,
			total_token_burn::text           AS total_token_burn,
			daily_nd_contract_token_burn::text AS daily_nd_contract_token_burn,
			total_nd_contract_token_burn::text AS total_nd_contract_token_burn,
			daily_poai_rewards::text         AS daily_poai_rewards,
			total_poai_rewards::text         AS total_poai_rewards,
			daily_minted::text               AS daily_minted,
			total_minted::text               AS total_minted,
			total_supply::text               AS total_supply,
			team_wallets_supply::text        AS team_wallets_supply,
			last_block_number
		`).
		Order("creation_timestamp DESC").
		Limit(1).
		Scan(&r).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return rowToModel(&r), nil
}

func updateStats(stats *model.Stats) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	row := map[string]any{
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

	res := db.Where("creation_timestamp = ?", stats.CreationTimestamp).Table("stats").Updates(row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("no row inserted")
	}
	return nil
}

func createStats(stats *model.Stats) error {
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

func toNumericExpr(x *big.Int) any {
	if x == nil {
		return nil
	}
	return gorm.Expr("?::numeric", x.String())
}

func toBigIntPtr(s *string) *big.Int {
	if s == nil {
		return nil
	}
	bi := new(big.Int)
	if _, ok := bi.SetString(*s, 10); !ok {
		return nil
	}
	return bi
}

func rowToModel(r *statsRow) *model.Stats {
	return &model.Stats{
		CreationTimestamp:        r.CreationTimestamp,
		DailyActiveJobs:          r.DailyActiveJobs,
		DailyUsdcLocked:          toBigIntPtr(r.DailyUsdcLocked),
		DailyTokenBurn:           toBigIntPtr(r.DailyTokenBurn),
		TotalTokenBurn:           toBigIntPtr(r.TotalTokenBurn),
		DailyNdContractTokenBurn: toBigIntPtr(r.DailyNdContractTokenBurn),
		TotalNdContractTokenBurn: toBigIntPtr(r.TotalNdContractTokenBurn),
		DailyPOAIRewards:         toBigIntPtr(r.DailyPOAIRewards),
		TotalPOAIRewards:         toBigIntPtr(r.TotalPOAIRewards),
		DailyMinted:              toBigIntPtr(r.DailyMinted),
		TotalMinted:              toBigIntPtr(r.TotalMinted),
		TotalSupply:              toBigIntPtr(r.TotalSupply),
		TeamWalletsSupply:        toBigIntPtr(r.TeamWalletsSupply),
		LastBlockNumber:          r.LastBlockNumber,
	}
}

func createUserInfo(userInfo *model.UserInfo) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&userInfo)
	if txCreate.Error != nil {
		txCreate.Rollback()
		return txCreate.Error
	}
	if txCreate.RowsAffected == 0 {
		txCreate.Rollback()
		return gorm.ErrRecordNotFound
	}

	return nil
}

func createAllocation(alloc *model.Allocation) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	txCreate := db.Create(&alloc)
	if txCreate.Error != nil {
		txCreate.Rollback()
		return txCreate.Error
	}
	if txCreate.RowsAffected == 0 {
		txCreate.Rollback()
		return gorm.ErrRecordNotFound
	}

	return nil
}
