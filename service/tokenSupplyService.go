package service

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type TokenSupplyInfo struct {
	Minted *big.Int
	Burned *big.Int
	Supply *big.Int
}

func GetTokenSupplyInfo(tokenAddress common.Address) (*TokenSupplyInfo, error) {
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return nil, errors.New("error while dialing client: " + err.Error())
	}
	defer client.Close()

	ctx := context.Background()

	latestBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return nil, errors.New("error while retrieving block number: " + err.Error())
	}

	fromBlock := big.NewInt(0)
	toBlock := big.NewInt(int64(latestBlock))

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

	mintedLogs, err := client.FilterLogs(ctx, mintedQuery)
	if err != nil {
		return nil, errors.New("error while filtering minted logs: " + err.Error())
	}

	mintedTotal := big.NewInt(0)
	for _, vLog := range mintedLogs {
		if len(vLog.Data) != 32 {
			return nil, errors.New("unexpected data length in minted log")
		}

		amount := new(big.Int).SetBytes(vLog.Data)
		mintedTotal.Add(mintedTotal, amount)
	}

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

	burnedLogs, err := client.FilterLogs(ctx, burnedQuery)
	if err != nil {
		return nil, errors.New("error while filtering burned logs: " + err.Error())
	}

	burnedTotal := big.NewInt(0)
	for _, vLog := range burnedLogs {
		if len(vLog.Data) != 32 {
			return nil, errors.New("unexpected data length in burned log")
		}

		amount := new(big.Int).SetBytes(vLog.Data)
		burnedTotal.Add(burnedTotal, amount)
	}

	const erc20ABI = `[{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	data, err := parsedABI.Pack("totalSupply")
	if err != nil {
		return nil, errors.New("error while packing interface: " + err.Error())
	}

	msg := ethereum.CallMsg{
		To:   &tokenAddress,
		Data: data,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, errors.New("error while calling contract: " + err.Error())
	}

	var totalSupply *big.Int
	err = parsedABI.UnpackIntoInterface(&totalSupply, "totalSupply", result)
	if err != nil {
		return nil, errors.New("error while unpacking interface: " + err.Error())
	}

	n := big.NewInt(0)
	if totalSupply.Int64() < n.Sub(mintedTotal, burnedTotal).Int64() {
		log.Error("total supply doesn't match minted and burned supply")
	}

	return &TokenSupplyInfo{
		Minted: mintedTotal,
		Burned: burnedTotal,
		Supply: totalSupply,
	}, nil
}
