package storage

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	once     sync.Once
	database *gorm.DB

	NoDBError = errors.New("no DB Connection")
)

func Connect() {
	once.Do(func() {
		sqlDb, err := sql.Open("postgres", config.Config.Database.Url())
		if err != nil {
			panic(err)
		}
		sqlDb.SetMaxOpenConns(config.Config.Database.MaxOpenConns)
		sqlDb.SetMaxIdleConns(config.Config.Database.MaxIdleConns)
		conn, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDb}))
		if err != nil {
			panic(err)
		}

		database = conn

		err = TryMigrate()
		if err != nil {
			panic(err)
		}
	})
}

func TryMigrate() error {
	err := database.AutoMigrate(
		&model.Account{},
		&model.Kyc{},
		&model.InvoiceClient{},
		&model.Seller{},
	)
	if err != nil {
		return err
	}
	return nil
}

func GetDB() (*gorm.DB, error) {
	if database == nil {
		return nil, NoDBError
	}

	return database, nil
}
