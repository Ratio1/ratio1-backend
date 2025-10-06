package main

import (
	"crypto/ecdsa"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	/*General*/
	SleepTime = 1 * time.Second
	/*Database connection*/
	once      sync.Once
	database  *gorm.DB
	NoDBError = errors.New("no DB Connection")

	Database = DatabaseConfig{
		User:         "postgres",
		Password:     "postgres",
		Host:         "localhost",
		Port:         5432,
		DbName:       "ratio1-db",
		MaxOpenConns: 100,
		MaxIdleConns: 100,
		SslMode:      "disable",
	}

	/*Sumsub connection*/
	SumsubAppToken  = ""
	SumsubSecretKey = ""

	/*Free currency api*/
	FreeCurrencyApiKey = ""

	/*Ratio1 oraclses auth*/
	sk             *ecdsa.PrivateKey
	PrivateKeyPath = ""

	/*Ethereum connection*/
	InfuraSecret = ""
	InfuraApiUrl = "https://base-mainnet.infura.io/v3/"

	/*Smart contracts*/
	OldAllocationEventSignature = "RewardsAllocated(uint256,address[],uint256)"
	AllocationEventSignature    = "RewardsAllocatedV2(uint256,address,address,uint256)"
	PoaiManagerAddress          = "0xa8d7FFCE91a888872A9f5431B4Dd6c0c135055c1"
	UsdcContractAddress         = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	NdContractAddress           = "0xE658DF6dA3FB5d4FBa562F1D5934bd0F9c6bd423"
	R1ContractAddress           = "0x6444C6c2D527D85EA97032da9A7504d6d1448ecF"
	TeamAddresses               = []string{
		"0xABdaAC00E36007fB71b2059fc0E784690a991923",
		"0x9a7055e3FBA00F5D5231994B97f1c0216eE1C091",
		"0x745C01f91c59000E39585441a3F1900AeF72c5C1",
		"0x5d5F16f1848c87b49185A9136cdF042384e82BA8",
		"0x0A27F805Db42089d79B96A4133A93B2e5Ff1b28C",
	}
	AllocLogsAbi = `[{
"anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "uint256",
          "name": "jobId",
          "type": "uint256"
        },
        {
          "indexed": false,
          "internalType": "address",
          "name": "nodeAddress",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "address",
          "name": "nodeOwner",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "usdcAmount",
          "type": "uint256"
        }
      ],
      "name": "RewardsAllocatedV2",
      "type": "event"
    }
]`

	OldAllocLogsAbi = `[{
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "uint256",
          "name": "jobId",
          "type": "uint256"
        },
        {
          "indexed": false,
          "internalType": "address[]",
          "name": "activeNodes",
          "type": "address[]"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "totalAmount",
          "type": "uint256"
        }
      ],
      "name": "RewardsAllocated",
      "type": "event"
    }
]`

	BurnEventSignature = "TokensBurned(uint256,uint256)"

	BurnLogsAbi = `
    [{
      "anonymous": false,
      "inputs": [
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "usdcAmount",
          "type": "uint256"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "r1Amount",
          "type": "uint256"
        }
      ],
      "name": "TokensBurned",
      "type": "event"
    }]
`
)
