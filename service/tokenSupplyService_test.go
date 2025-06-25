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
	teamSupply, err := GetTeamWalletsSupply()
	require.Nil(t, err)
	circulatingSupply := totalSupply - teamSupply
	fmt.Println("Circulating Supply:", circulatingSupply)
	fmt.Println("Total Supply:", totalSupply)
	fmt.Println("Total Minted:", totalMinted)
	fmt.Println("Total Burned:", totalBurned)
	fmt.Println("Team Supply:", teamSupply)
}
