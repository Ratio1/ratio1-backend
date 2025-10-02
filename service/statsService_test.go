package service

import (
	"fmt"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
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
