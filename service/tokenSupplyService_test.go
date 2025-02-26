package service

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func Test_GetTokenSupply(t *testing.T) {
	address := common.HexToAddress("0xc992dcab6d3f8783fbf0c935e7bceb20aa50a6f1")
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	require.Nil(t, err)
	defer client.Close()

	latestBlock, err := GetLastBlockNumber(client)
	require.Nil(t, err)
	totalSupply, err := GetTotalSupply(client, latestBlock, address)
	require.Nil(t, err)
	totalMinted, err := GetTotalMintedAmount(client, latestBlock, address)
	require.Nil(t, err)
	totalBurned, err := GetTotalBurnedAmount(client, latestBlock, address)
	require.Nil(t, err)

	oneToken := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)
	trimmedSuply := big.NewInt(0).Div(totalSupply, oneToken)
	trimmedMinted := big.NewInt(0).Div(totalMinted, oneToken)
	trimmedBurned := big.NewInt(0).Div(totalBurned, oneToken)
	fmt.Println(trimmedSuply, trimmedMinted, trimmedBurned)
}
