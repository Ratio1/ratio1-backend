package service

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/ratio1abi"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func GetAmountAsFloatString(amount *big.Int, decimals int) string {
	if amount == nil {
		return ""
	}

	oneToken := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	amountFloat := new(big.Float).SetInt(amount)
	amountFloat.Quo(amountFloat, new(big.Float).SetInt(oneToken))
	return amountFloat.Text('f', 18)
}

func GetAmountAsFloat(amount *big.Int, decimals int) float64 {
	if amount == nil {
		return 0
	}

	oneToken := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	amountFloat := new(big.Float).SetInt(amount)
	amountFloat.Quo(amountFloat, new(big.Float).SetInt(oneToken))
	v, _ := amountFloat.Float64()
	return v
}

func CalcCircSupply(teamSupply, totalSupply string) string {
	teamSupplyBig, tmOk := new(big.Float).SetString(teamSupply)
	totalSupplyBig, ttOk := new(big.Float).SetString(totalSupply)
	if !tmOk && !ttOk {
		return "0"
	} else if !ttOk {
		return teamSupply
	} else if !tmOk {
		return totalSupply
	}
	circSupply := new(big.Float).Sub(totalSupplyBig, teamSupplyBig)
	return circSupply.Text('f', 18)
}

func getPeriodMintedAmount(from, to int64) (*big.Int, error) {
	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return big.NewInt(0), errors.New("error while dialing client")
	}
	defer client.Close()

    transferEventSignature := []byte(ratio1abi.TransferEventSignature)
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	mintedTotal := big.NewInt(0)
	alreadySeen := make(map[string]bool)

	for {
		mintedQuery := ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{tokenAddress},
			Topics: [][]common.Hash{
				{transferEventSigHash},
				{zeroTopic}, // This is the "to" address, which we don't filter on
			},
		}

		mintedLogs, err := client.FilterLogs(context.Background(), mintedQuery)
		if err != nil {
			return big.NewInt(0), errors.New("error while filtering minted logs")
		}

		for _, vLog := range mintedLogs {
			if _, exist := alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))]; exist {
				continue
			}

			if len(vLog.Data) != 32 {
				return big.NewInt(0), errors.New("unexpected data length in minted log")
			}

			amount := new(big.Int).SetBytes(vLog.Data)
			mintedTotal.Add(mintedTotal, amount)
			alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))] = true

			if vLog.BlockNumber > fromBlock.Uint64() {
				fromBlock = big.NewInt(int64(vLog.BlockNumber))
			}
		}

		if len(mintedLogs) < 10000 {
			break
		}
	}

	return mintedTotal, nil
}

func getPeriodBurnedAmount(from, to int64) (*big.Int, error) {
	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return big.NewInt(0), errors.New("error while dialing client: " + err.Error())
	}
	defer client.Close()

    transferEventSignature := []byte(ratio1abi.TransferEventSignature)
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	burnedTotal := big.NewInt(0)
	alreadySeen := make(map[string]bool)

	for {
		burnedQuery := ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{tokenAddress},
			Topics: [][]common.Hash{
				{transferEventSigHash},
				{},
				{zeroTopic},
			},
		}

		burnedLogs, err := client.FilterLogs(context.Background(), burnedQuery)
		if err != nil {
			return big.NewInt(0), errors.New("error while filtering burned logs: " + err.Error())
		}

		for _, vLog := range burnedLogs {
			if _, exist := alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))]; exist {
				continue
			}

			if len(vLog.Data) != 32 {
				return big.NewInt(0), errors.New("unexpected data length in burned log")
			}

			amount := new(big.Int).SetBytes(vLog.Data)
			burnedTotal.Add(burnedTotal, amount)
			alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))] = true

			if vLog.BlockNumber > fromBlock.Uint64() {
				fromBlock = big.NewInt(int64(vLog.BlockNumber))
			}
		}

		if len(burnedLogs) < 10000 {
			break
		}
	}

	return burnedTotal, nil
}

func getPeriodNdContractBurnedAmount(from, to int64) (*big.Int, error) {
	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return big.NewInt(0), errors.New("error while dialing client: " + err.Error())
	}
	defer client.Close()

    transferEventSignature := []byte(ratio1abi.TransferEventSignature)
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	ndContractAddress := common.HexToAddress(config.Config.NDContractAddress)
	ndContractTopic := common.BytesToHash(ndContractAddress.Bytes())

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	burnedTotal := big.NewInt(0)
	alreadySeen := make(map[string]bool)

	for {
		burnedQuery := ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{tokenAddress},
			Topics: [][]common.Hash{
				{transferEventSigHash},
				{ndContractTopic},
				{zeroTopic},
			},
		}

		burnedLogs, err := client.FilterLogs(context.Background(), burnedQuery)
		if err != nil {
			return big.NewInt(0), errors.New("error while filtering burned logs: " + err.Error())
		}

		for _, vLog := range burnedLogs {
			if _, exist := alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))]; exist {
				continue
			}

			if len(vLog.Data) != 32 {
				return big.NewInt(0), errors.New("unexpected data length in burned log")
			}

			amount := new(big.Int).SetBytes(vLog.Data)
			burnedTotal.Add(burnedTotal, amount)
			alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))] = true

			if vLog.BlockNumber > fromBlock.Uint64() {
				fromBlock = big.NewInt(int64(vLog.BlockNumber))
			}
		}

		if len(burnedLogs) < 10000 {
			break
		}
	}

	return burnedTotal, nil
}

func getTotalSupply() (*big.Int, error) {

	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)

    parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.Erc20ABI))
	if err != nil {
		return big.NewInt(0), errors.New("error while parsing abi: " + err.Error())
	}

	data, err := parsedABI.Pack("totalSupply")
	if err != nil {
		return big.NewInt(0), errors.New("error while packing interface: " + err.Error())
	}

	msg := ethereum.CallMsg{
		To:   &tokenAddress,
		Data: data,
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return big.NewInt(0), errors.New("error while dialing client: " + err.Error())
	}
	defer client.Close()

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return big.NewInt(0), errors.New("error while calling contract: " + err.Error())
	}

	var totalSupply *big.Int
	err = parsedABI.UnpackIntoInterface(&totalSupply, "totalSupply", result)
	if err != nil {
		return big.NewInt(0), errors.New("error while unpacking interface: " + err.Error())
	}

	return totalSupply, nil
}

func getTeamWalletsSupply() (*big.Int, error) {
	tokenAddress := common.HexToAddress(config.Config.R1ContractAddress)

    parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.Erc20ABI))
	if err != nil {
		return big.NewInt(0), errors.New("error while parsing abi: " + err.Error())
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return big.NewInt(0), errors.New("error while dialing client: " + err.Error())
	}
	defer client.Close()

	var totalTeamBalance = big.NewInt(0)

	for _, addrStr := range config.Config.TeamAddresses {
		teamAddress := common.HexToAddress(addrStr)

		// Pack balanceOf call
		balanceData, err := parsedABI.Pack("balanceOf", teamAddress)
		if err != nil {
			return big.NewInt(0), errors.New("error packing balanceOf: " + err.Error())
		}

		msg := ethereum.CallMsg{
			To:   &tokenAddress,
			Data: balanceData,
		}

		result, err := client.CallContract(context.Background(), msg, nil)
		if err != nil {
			return big.NewInt(0), errors.New("error calling balanceOf for " + addrStr)
		}

		var balance *big.Int
		err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
		if err != nil {
			return big.NewInt(0), errors.New("error unpacking balanceOf for " + addrStr + ": " + err.Error())
		}

		totalTeamBalance = totalTeamBalance.Add(totalTeamBalance, balance)
	}

	return totalTeamBalance, nil
}
