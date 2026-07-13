package service

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
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

func TestOrphanedAllocationOwnerOverride(t *testing.T) {
	matchingAllocation := model.Allocation{
		BlockNumber: 48325341,
		TxHash:      "0x76c49d7f85eaabd39f421d892bd494e74786e476e348ced8e9b4e6564fbe18d1",
		NodeAddress: "0x3795d06dcd5cb35E25E669978e51c1C60c5105bC",
		CspAddress:  "0x0b81c24153bFbB2C98813da8Ac0EF7e8b83Ba389",
	}

	tests := []struct {
		name          string
		allocation    model.Allocation
		resolvedOwner string
		expectedOwner string
		overridden    bool
	}{
		{
			name:          "overrides matching unresolved allocation",
			allocation:    matchingAllocation,
			resolvedOwner: "0x0000000000000000000000000000000000000000",
			expectedOwner: orphanedAllocationOwner,
			overridden:    true,
		},
		{
			name:          "overrides matching allocation after node is relinked",
			allocation:    matchingAllocation,
			resolvedOwner: "0x1111111111111111111111111111111111111111",
			expectedOwner: orphanedAllocationOwner,
			overridden:    true,
		},
		{
			name: "rejects wrong block",
			allocation: func() model.Allocation {
				allocation := matchingAllocation
				allocation.BlockNumber++
				return allocation
			}(),
			resolvedOwner: "0x0000000000000000000000000000000000000000",
			expectedOwner: "0x0000000000000000000000000000000000000000",
		},
		{
			name: "rejects unrelated node",
			allocation: func() model.Allocation {
				allocation := matchingAllocation
				allocation.NodeAddress = "0x2222222222222222222222222222222222222222"
				return allocation
			}(),
			resolvedOwner: "0x0000000000000000000000000000000000000000",
			expectedOwner: "0x0000000000000000000000000000000000000000",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			owner, overridden := orphanedAllocationOwnerOverride(test.allocation, test.resolvedOwner)
			require.Equal(t, test.expectedOwner, owner)
			require.Equal(t, test.overridden, overridden)
		})
	}
}

func TestOrphanedAllocationOwnerOverrideCoversAffectedEvents(t *testing.T) {
	nodes := []string{
		"0x3795d06dcd5cb35E25E669978e51c1C60c5105bC",
		"0x3fA6b8254057670C8Ac0673F3ABFfa277E07f88f",
		"0xBCA7A87C4730Fb7e04e7858CD0152eea65B82492",
		"0xcA35C471Dbb7296384EC448e8Dc1757B0715200F",
	}
	for txHash, blockNumber := range orphanedAllocationBlocksByTx {
		for _, nodeAddress := range nodes {
			allocation := model.Allocation{
				BlockNumber: blockNumber,
				TxHash:      txHash,
				NodeAddress: nodeAddress,
				CspAddress:  orphanedAllocationCsp,
			}
			owner, overridden := orphanedAllocationOwnerOverride(allocation, common.Address{}.String())
			require.True(t, overridden)
			require.Equal(t, orphanedAllocationOwner, owner)
		}
	}
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
		Address:     common.HexToAddress(orphanedAllocationCsp),
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
