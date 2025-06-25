package service

import (
	"context"
	"errors"
	"math/big"
	"strconv"
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
	timestamp       time.Time //last time this value was updated
	value           int64     //last value saved
	lastBlockNumber int64     //last block number seen
}

var (
	SupplyKey      = "supply"
	MintedKey      = "minted"
	BurnedKey      = "burned"
	TeamWalletsKey = "team_wallets_supply"
)
var tokenSupplyData = make(map[string]supplyData)
var mu sync.Mutex
var oneToken = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)

func GetTotalMintedAmount() (int64, error) {
	var fromBlock *big.Int
	startingValue := int64(0)

	if valid, value, lastBlock := getFromSupplyData(MintedKey); valid {
		return value, nil
	} else {
		startingValue += value
		fromBlock = big.NewInt(lastBlock)
	}

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client")
	}
	defer client.Close()

	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	mintedTotal := big.NewInt(0)
	alreadySeen := make(map[string]bool)

	for {
		mintedQuery := ethereum.FilterQuery{
			FromBlock: fromBlock,
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

		for _, vLog := range mintedLogs {
			if _, exist := alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))]; exist {
				continue
			}

			if len(vLog.Data) != 32 {
				return 0, errors.New("unexpected data length in minted log")
			}

			amount := new(big.Int).SetBytes(vLog.Data)
			mintedTotal.Add(mintedTotal, amount)
			alreadySeen[vLog.TxHash.String()] = true

			if vLog.BlockNumber > fromBlock.Uint64() {
				fromBlock = big.NewInt(int64(vLog.BlockNumber))
			}
		}

		if len(mintedLogs) < 10000 {
			break
		}
	}

	trimmedMinted := big.NewInt(0).Div(mintedTotal, oneToken)
	setInSupplyData(MintedKey, supplyData{
		timestamp:       time.Now(),
		value:           trimmedMinted.Int64() + startingValue,
		lastBlockNumber: fromBlock.Int64(),
	})

	return trimmedMinted.Int64(), nil
}

func GetTotalBurnedAmount() (int64, error) {
	var fromBlock *big.Int
	startingValue := int64(0)

	if valid, value, lastBlock := getFromSupplyData(BurnedKey); valid {
		return value, nil
	} else {
		startingValue += value
		fromBlock = big.NewInt(lastBlock)
	}

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client")
	}
	defer client.Close()

	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	burnedTotal := big.NewInt(0)
	alreadySeen := make(map[string]bool)

	for {
		burnedQuery := ethereum.FilterQuery{
			FromBlock: fromBlock,
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

		for _, vLog := range burnedLogs {
			if _, exist := alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))]; exist {
				continue
			}

			if len(vLog.Data) != 32 {
				return 0, errors.New("unexpected data length in burned log")
			}

			amount := new(big.Int).SetBytes(vLog.Data)
			burnedTotal.Add(burnedTotal, amount)
			alreadySeen[vLog.TxHash.String()] = true

			if vLog.BlockNumber > fromBlock.Uint64() {
				fromBlock = big.NewInt(int64(vLog.BlockNumber))
			}
		}

		if len(burnedLogs) < 10000 {
			break
		}
	}

	trimmedBurned := big.NewInt(0).Div(burnedTotal, oneToken)
	setInSupplyData(BurnedKey, supplyData{
		timestamp:       time.Now(),
		value:           trimmedBurned.Int64() + startingValue,
		lastBlockNumber: fromBlock.Int64(),
	})

	return trimmedBurned.Int64(), nil
}

func GetTotalSupply() (int64, error) {
	if valid, value, _ := getFromSupplyData(SupplyKey); valid {
		return value, nil
	}

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)

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

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client")
	}
	defer client.Close()

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

func GetTeamWalletsSupply() (int64, error) {
	if valid, value, _ := getFromSupplyData(TeamWalletsKey); valid {
		return value, nil
	}

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)

	const erc20ABI = `[{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"},
	{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return 0, errors.New("error while parsing abi: " + err.Error())
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client")
	}
	defer client.Close()

	var totalTeamBalance = big.NewInt(0)

	for _, addrStr := range config.Config.TeamAddresses {
		teamAddress := common.HexToAddress(addrStr)

		// Pack balanceOf call
		balanceData, err := parsedABI.Pack("balanceOf", teamAddress)
		if err != nil {
			return 0, errors.New("error packing balanceOf: " + err.Error())
		}

		msg := ethereum.CallMsg{
			To:   &tokenAddress,
			Data: balanceData,
		}

		result, err := client.CallContract(context.Background(), msg, nil)
		if err != nil {
			return 0, errors.New("error calling balanceOf for " + addrStr)
		}

		var balance *big.Int
		err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
		if err != nil {
			return 0, errors.New("error unpacking balanceOf for " + addrStr + ": " + err.Error())
		}

		totalTeamBalance = totalTeamBalance.Add(totalTeamBalance, balance)
	}

	trimmedSupply := big.NewInt(0).Div(totalTeamBalance, oneToken)
	setInSupplyData(TeamWalletsKey, supplyData{
		timestamp: time.Now(),
		value:     trimmedSupply.Int64(),
	})

	return trimmedSupply.Int64(), nil
}

func getFromSupplyData(key string) (isValid bool, value, blockNumber int64) {
	mu.Lock()
	defer mu.Unlock()
	if v, found := tokenSupplyData[key]; found {
		if time.Since(v.timestamp).Minutes() <= 10 {
			return true, v.value, v.lastBlockNumber
		} else {
			return false, v.value, v.lastBlockNumber
		}
	} else {
		return false, 0, 0
	}
}

func setInSupplyData(key string, value supplyData) {
	mu.Lock()
	defer mu.Unlock()
	tokenSupplyData[key] = value
}
