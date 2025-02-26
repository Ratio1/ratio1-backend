package service

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type supplyData struct {
	timestamp time.Time
	value     int64
}

var (
	SupplyKey = "supply"
	MintedKey = "minted"
	BurnedKey = "burned"
)
var tokenSupplyData = make(map[string]supplyData)
var mu sync.Mutex
var oneToken = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)

func GetTotalMintedAmount() (int64, error) {
	if valid, value := getFromSupplyData(MintedKey); valid {
		return value, nil
	}

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client")
	}
	defer client.Close()

	latestBlock, err := getLastBlockNumber(client)
	if err != nil {
		return 0, err
	}

	fromBlock := big.NewInt(0)
	toBlock := big.NewInt(latestBlock)

	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	mintedQuery := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []common.Address{tokenAddress},
		Topics: [][]common.Hash{
			{transferEventSigHash},
			{zeroTopic},
		},
	}

	mintedLogs, err := client.FilterLogs(context.Background(), mintedQuery)
	if err != nil {
		return 0, errors.New("error while filtering minted logs")
	}

	mintedTotal := big.NewInt(0)
	for _, vLog := range mintedLogs {
		if len(vLog.Data) != 32 {
			return 0, errors.New("unexpected data length in minted log")
		}

		amount := new(big.Int).SetBytes(vLog.Data)
		mintedTotal.Add(mintedTotal, amount)
	}

	trimmedMinted := big.NewInt(0).Div(mintedTotal, oneToken)
	setInSupplyData(MintedKey, supplyData{
		timestamp: time.Now(),
		value:     trimmedMinted.Int64(),
	})

	return trimmedMinted.Int64(), nil
}

func GetTotalBurnedAmount() (int64, error) {
	if valid, value := getFromSupplyData(BurnedKey); valid {
		return value, nil
	}
	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client")
	}
	defer client.Close()

	latestBlock, err := getLastBlockNumber(client)
	if err != nil {
		return 0, err
	}

	fromBlock := big.NewInt(0)
	toBlock := big.NewInt(latestBlock)

	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	burnedQuery := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []common.Address{tokenAddress},
		Topics: [][]common.Hash{
			{transferEventSigHash},
			nil,
			{zeroTopic},
		},
	}

	burnedLogs, err := client.FilterLogs(context.Background(), burnedQuery)
	if err != nil {
		return 0, errors.New("error while filtering burned logs")
	}

	burnedTotal := big.NewInt(0)
	for _, vLog := range burnedLogs {
		if len(vLog.Data) != 32 {
			return 0, errors.New("unexpected data length in burned log")
		}

		amount := new(big.Int).SetBytes(vLog.Data)
		burnedTotal.Add(burnedTotal, amount)
	}

	trimmedBurned := big.NewInt(0).Div(burnedTotal, oneToken)
	setInSupplyData(BurnedKey, supplyData{
		timestamp: time.Now(),
		value:     trimmedBurned.Int64(),
	})

	return trimmedBurned.Int64(), nil
}

func GetTotalSupply() (int64, error) {
	if valid, value := getFromSupplyData(SupplyKey); valid {
		return value, nil
	}
	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client")
	}
	defer client.Close()

	const erc20ABI = `[{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return 0, errors.New("error while parsing abi: " + err.Error())
	}

	data, err := parsedABI.Pack("totalSupply")
	if err != nil {
		return 0, errors.New("error while packing interface: " + err.Error())
	}

	msg := ethereum.CallMsg{
		To:   &tokenAddress,
		Data: data,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return 0, errors.New("error while calling contract")
	}

	var totalSupply *big.Int
	err = parsedABI.UnpackIntoInterface(&totalSupply, "totalSupply", result)
	if err != nil {
		return 0, errors.New("error while unpacking interface: " + err.Error())
	}

	trimmedSupply := big.NewInt(0).Div(totalSupply, oneToken)
	setInSupplyData(SupplyKey, supplyData{
		timestamp: time.Now(),
		value:     trimmedSupply.Int64(),
	})

	return trimmedSupply.Int64(), nil
}

func getLastBlockNumber(client *ethclient.Client) (int64, error) {
	latestBlock, err := client.BlockNumber(context.Background())
	if err != nil {
		return 0, errors.New("error while retrieving block number")
	}
	return int64(latestBlock), nil
}

func getFromSupplyData(key string) (isValid bool, value int64) {
	mu.Lock()
	defer mu.Unlock()
	if v, found := tokenSupplyData[key]; found {
		if time.Since(v.timestamp).Minutes() <= 10 {
			return true, v.value
		} else {
			return false, 0
		}
	} else {
		return false, 0
	}
}

func setInSupplyData(key string, value supplyData) {
	mu.Lock()
	defer mu.Unlock()
	tokenSupplyData[key] = value
}
