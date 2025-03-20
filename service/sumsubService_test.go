package service

import (
	"fmt"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/stretchr/testify/require"
)

func TestGetSessionInfo(t *testing.T) {
	token, err := InitNewSession("10766129-e2b490f9-3f85f800-237144da", config.Config.Sumsub.CustomerLevelName)
	require.Nil(t, err)
	fmt.Println(*token)
}

func TestGetUserInfo(t *testing.T) {
	token, err := GetClientInfos("67a4c00eb60be71a0103b7d5", "e73918b9-fc53-411a-a4c2-ec0058437b93")
	require.Nil(t, err)
	fmt.Println(*token)
}
