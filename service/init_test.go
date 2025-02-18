package service

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
)

func init() {
	config.Config.Sumsub = config.SumsubConfig{
		ApiUrl:            "https://api.sumsub.com",
		ApiEndpoint:       "/resources/accessTokens/sdk",
		CustomerLevelName: "id-and-liveness",
		BusinessLevelName: "",
		SumsubAppToken:    "sbx:hY0sgo3KXMysg5RbfsdPUwv8.pNpFk7CxieuHEUYhpstmVt04Dp6yJj9E",
		SumsubSecretKey:   "lekdpKm52MwU98VZARYtQL3wII3m3wye",
	}

	/*config.Config.Database = config.DatabaseConfig{
		User:         "r1backend",
		Password:     "Th3B35tB@ck3nd",
		Host:         "51.159.25.105",
		Port:         22992,
		DbName:       "rdb",
		MaxOpenConns: 100,
		MaxIdleConns: 50,
		SslMode:      "disable",
	}*/
	config.Config.Database = config.DatabaseConfig{
		User:         "postgres",
		Password:     "postgres",
		Host:         "localhost",
		Port:         5432,
		DbName:       "ratio1-db",
		MaxOpenConns: 100,
		MaxIdleConns: 100,
		SslMode:      "disable",
	}

	config.Config.Jwt = config.JwtConfig{
		Issuer:            "localhost:5000",
		KeySeedHex:        "d6592724167553acf9c8cba9a7dbc7f514efc757d7906546cecfdfc5d4d3e8d1",
		Secret:            "a5aa6a0ead4b1c60a6e23ef3a97f8bf1e6d712debd0ecd516acfcfc5d177d1e4",
		ConfirmSecret:     "jwtConfirmSecret",
		ConfirmExpiryMins: 1440,
	}
	config.Config.EmailTemplatesPath = "../templates/html/"
	config.Config.Oblio.AuthUrl = "https://www.oblio.eu/api/authorize/token"
	config.Config.Oblio.InvoiceUrl = "https://www.oblio.eu/api/docs/invoice"
	config.Config.Oblio.ClientSecret = "client_id=dev@ratio1.ai&client_secret=2eb13c437f8426f8e605f8e26c8ebe2b084f0768"
	config.Config.Oblio.EventSignature = "LicensesCreated(address,bytes32,uint256,uint256,uint256)"
	config.Config.NDContractAddress = "0x0421b7c9A3B1a4f99F56131b65d15085C7cCACB0"
	config.Config.Infura.Secret = "533c2b6ac99b4f11b513d25cfb5dffd1"
	config.Config.Infura.ApiUrl = "https://base-sepolia.infura.io/v3/"
	config.Config.Oblio.Abi = `[{
		"anonymous": false,
		"inputs": [
		  {
			"indexed": true,
			"internalType": "address",
			"name": "to",
			"type": "address"
		  },
		  {
			"indexed": true,
			"internalType": "bytes32",
			"name": "invoiceUuid",
			"type": "bytes32"
		  },
		  {
			"indexed": false,
			"internalType": "uint256",
			"name": "tokenCount",
			"type": "uint256"
		  },
		  {
			"indexed": false,
			"internalType": "uint256",
			"name": "unitUsdPrice",
			"type": "uint256"
		  },
		  {
			"indexed": false,
			"internalType": "uint256",
			"name": "totalR1Spent",
			"type": "uint256"
		  }
		],
		"name": "LicensesCreated",
		"type": "event"
  }]`

	config.Config.BuyLimitUSD.Individual = 10000
	config.Config.BuyLimitUSD.Company = 200000

	storage.Connect()
}
