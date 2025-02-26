package service

import (
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/stretchr/testify/require"
)

func Test_ViesIntegration(t *testing.T) {
	config.Config.ViesApi = config.ViesConfig{
		BaseUrl:  "https://viesapi.eu/api-test",
		User:     "test_id",
		Password: "test_key",
	}
	isValid := IsCompanyRegistered("POL", "7171642051")
	require.True(t, isValid)
}
