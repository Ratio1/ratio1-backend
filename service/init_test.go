package service

import (
	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
)

func init() {
	config.Config.Sumsub = config.SumsubConfig{
		ApiUrl:            "https://api.sumsub.com",
		ApiEndpoint:       "/resources/accessTokens/sdk",
		CustomerLevelName: "",
		BusinessLevelName: "",
		SumsubAppToken:    "",
		SumsubSecretKey:   "",
	}

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

	config.Config.EmailTemplatesPath = "../templates/html/"
	config.Config.Oblio.AuthUrl = "https://www.oblio.eu/api/authorize/token"
	config.Config.Oblio.InvoiceUrl = "https://www.oblio.eu/api/docs/invoice"
	config.Config.Oblio.ClientSecret = ""
	config.Config.Oblio.EventSignature = "LicensesCreated(address,bytes32,uint256,uint256,uint256)"
	config.Config.NDContractAddress = "0xE658DF6dA3FB5d4FBa562F1D5934bd0F9c6bd423"
	config.Config.R1ContractAddress = "0x6444C6c2D527D85EA97032da9A7504d6d1448ecF"
	config.Config.Infura.Secret = "533c2b6ac99b4f11b513d25cfb5dffd1" //test secret, test use only
	config.Config.Infura.ApiUrl = "https://base-mainnet.infura.io/v3/"
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

	//storage.Connect()
}
