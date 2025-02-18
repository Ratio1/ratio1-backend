package process

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetEgldPrice(t *testing.T) {
	price, err := GetEgldPrice()
	require.Nil(t, err)
	t.Log(price)
}
