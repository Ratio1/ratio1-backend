package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_GetTokenSupply(t *testing.T) {
	totalSupply, err := GetTotalSupply()
	require.Nil(t, err)
	totalMinted, err := GetTotalMintedAmount()
	require.Nil(t, err)
	totalBurned, err := GetTotalBurnedAmount()
	require.Nil(t, err)
	fmt.Println(totalSupply, totalMinted, totalBurned)
}
