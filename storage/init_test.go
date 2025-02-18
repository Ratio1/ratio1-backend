package storage

import "github.com/NaeuralEdgeProtocol/ratio1-backend/config"

var dbConfig = config.DatabaseConfig{}

func init() {
	config.Config.Database = dbConfig
	Connect()
}
