package service

import (
	"fmt"
	"math/big"
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
	circulatingSupply := big.NewInt(0).Sub(totalSupply, teamSupply)
	ndContractBurn, err := GetNdContractTotalBurnedAmount()
	require.Nil(t, err)
	fmt.Println("Circulating Supply:", circulatingSupply)
	fmt.Println("Total Supply:", totalSupply)
	fmt.Println("Total Minted:", totalMinted)
	fmt.Println("Total Burned:", totalBurned)
	fmt.Println("Team Supply:", teamSupply)
	fmt.Println("ND Contract Burn:", ndContractBurn)
	/* print as big float*/

	fmt.Println("Circulating Supply (float):", CalcCircSupply(GetAmountAsFloatString(teamSupply), GetAmountAsFloatString(totalSupply)))
	fmt.Println("Total Supply (float):", GetAmountAsFloatString(totalSupply))
	fmt.Println("Total Minted (float):", GetAmountAsFloatString(totalMinted))
	fmt.Println("Total Burned (float):", GetAmountAsFloatString(totalBurned))
	fmt.Println("Team Supply (float):", GetAmountAsFloatString(teamSupply))
	fmt.Println("ND Contract Burn (float):", GetAmountAsFloatString(ndContractBurn))
}
