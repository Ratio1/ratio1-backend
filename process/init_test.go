package process

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
)

func init() {
	config.Config.Database = config.DatabaseConfig{
		User:         "postgres",
		Password:     "root",
		Host:         "localhost",
		Port:         5432,
		DbName:       "launchpad-db",
		MaxOpenConns: 50,
		MaxIdleConns: 10,
		SslMode:      "disable",
	}

	storage.Connect()
}
