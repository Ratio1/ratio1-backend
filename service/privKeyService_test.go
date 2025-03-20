package service

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetBackendPrivKey(t *testing.T) {

	expectedAddress := "0x7baD7944E5d43CD50c14F4D159e706a68E8Ffe09"
	expectedPubKey := "04b28e62e2a8d9f9d83c026b307c2be171156f3b05eca328806714c5370998dfbd8462e03400d20a618708f1b7f9b696d0e780d78896d2abbf5fe6525ef98611fa"

	privateKey, err := GetBackendPrivKey()
	require.Nil(t, err)

	ethAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	fmt.Println(ethAddress)
	publicKeyBytes := crypto.FromECDSAPub(&privateKey.PublicKey)
	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	assert.Equal(t, expectedAddress, ethAddress.String())
	assert.Equal(t, expectedPubKey, publicKeyHex)
}
