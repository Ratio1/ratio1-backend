package service

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/ratio1abi"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func Test_GetDailyStats(t *testing.T) {
	DailyGetStats() //MAKE SURE TO HAVE A DB CONNECTED
}

func Test_GetDailyUsdcLocked(t *testing.T) {
	config.Config.Infura.Secret = "533c2b6ac99b4f11b513d25cfb5dffd1" //test secret, test use only
	config.Config.Infura.ApiUrl = "https://base-mainnet.infura.io/v3/"
	value, err := getDailyUsdcLocked()
	require.Nil(t, err)
	fmt.Println(value.String())
}

func TestDecodeAllocLogsPersistsLogIndex(t *testing.T) {
	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.AllocationLogsAbi))
	require.NoError(t, err)
	event := parsedABI.Events["RewardsAllocatedV3"]
	data, err := event.Inputs.NonIndexed().Pack(
		common.HexToAddress("0x3795d06dcd5cb35E25E669978e51c1C60c5105bC"),
		big.NewInt(318750),
	)
	require.NoError(t, err)

	allocation, err := decodeAllocLogs(types.Log{
		Address:     common.HexToAddress("0x0b81c24153bFbB2C98813da8Ac0EF7e8b83Ba389"),
		Topics:      []common.Hash{event.ID, common.BigToHash(big.NewInt(30))},
		Data:        data,
		TxHash:      common.HexToHash("0x76c49d7f85eaabd39f421d892bd494e74786e476e348ced8e9b4e6564fbe18d1"),
		BlockNumber: 48325341,
		Index:       563,
	})
	require.NoError(t, err)
	require.NotNil(t, allocation.LogIndex)
	require.Equal(t, uint(563), *allocation.LogIndex)
}
