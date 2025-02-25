package service

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func Test_GetTokenSupply(t *testing.T) {
	address := common.HexToAddress("0xc992dcab6d3f8783fbf0c935e7bceb20aa50a6f1")
	supply, err := GetTokenSupplyInfo(address)
	require.Nil(t, err)
	fmt.Println(supply)
	oneToken := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)
	trimmedSuply := big.NewInt(0).Div(supply.Supply, oneToken)
	trimmedMinted := big.NewInt(0).Div(supply.Minted, oneToken)
	trimmedBurned := big.NewInt(0).Div(supply.Burned, oneToken)
	fmt.Println(trimmedSuply, trimmedMinted, trimmedBurned)
}
