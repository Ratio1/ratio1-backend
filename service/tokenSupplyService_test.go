package service

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func Test_GetTokenSupply(t *testing.T) {
	address := common.HexToAddress("0xc992dcab6d3f8783fbf0c935e7bceb20aa50a6f1")
	resp, err := GetTokenSupplyInfo(address)
	require.Nil(t, err)
	fmt.Println(resp)
}
