package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_IncreaseCountMapOverLimit(t *testing.T) {
	address := "erd1"

	for i := 1; i < maxEmailsPerAddress; i++ {
		err := increaseEmailCount(address)
		require.Nil(t, err)
	}

	err := increaseEmailCount(address)
	require.NotNil(t, err)
}
