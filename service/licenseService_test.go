package service

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Claim struct {
	NodeAddress    string
	Epochs         []int
	Availabilities []int
}

const vat = int64(19)

/*
func padTo32Bytes(b []byte) []byte {
	if len(b) < 32 {
		// Pad the byte slice to the left with zeros
		padding := make([]byte, 32-len(b))
		return append(padding, b...)
	}
	return b
}*/

func Test_signByte(t *testing.T) {
	claim := Claim{
		NodeAddress:    "0x5FbDB2315678afecb367f032d93F642f64180aa3",
		Epochs:         []int{1, 2, 3, 4, 5},
		Availabilities: []int{250, 130, 178, 12, 0},
	}

	nodeAddress := common.HexToAddress(claim.NodeAddress)

	epochs := make([]*big.Int, len(claim.Epochs))
	for i, epoch := range claim.Epochs {
		epochs[i] = big.NewInt(int64(epoch))
	}

	availabilities := make([]*big.Int, len(claim.Availabilities))
	for i, availability := range claim.Availabilities {
		availabilities[i] = big.NewInt(int64(availability))
	}

	var packedData []byte

	packedData = append(packedData, nodeAddress.Bytes()...)

	for _, epoch := range epochs {
		packedData = append(packedData, padTo32Bytes(epoch.Bytes())...)
	}

	for _, availability := range availabilities {
		packedData = append(packedData, padTo32Bytes(availability.Bytes())...)
	}

	privKey, err := GetBackendPrivKey()
	require.Nil(t, err)

	fmt.Println(crypto.PubkeyToAddress(privKey.PublicKey))

	hash := crypto.Keccak256Hash(packedData)
	ethSigner := crypto.Keccak256Hash(
		[]byte("\x19Ethereum Signed Message:\n32"),
		hash.Bytes(),
	)
	signature, err := crypto.Sign(ethSigner.Bytes(), privKey)
	require.Nil(t, err)
	if signature[64] < 27 {
		signature[64] += 27
	}
	fmt.Println(hex.EncodeToString(signature))

}

func Test_NewBuyLicenseTxTemplate(t *testing.T) {
	uuid := "d18ac3989ae74da398c8ab26de41bb7c"
	walletAddress := "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
	resp, err := NewBuyLicenseTxTemplate(walletAddress, uuid, 10000, vat)
	require.Nil(t, err)
	fmt.Println(resp)
	//expectedSignature := "0bbb76f330fe36625b3c932055b5e7b5a7adb86b1e19c727cf21f8ada45299a97d35232bbc3205663b610ae2f3e2017eecc6ad62f7b22afa846762d666bb6ec81b"
	//assert.Equal(t, expectedSignature, resp)
}

func Test_ConstructAndSignClaim(t *testing.T) {
	uuid := "d18ac398-9ae7-4da3-98c8-ab26de41bb7c"
	privKey, err := GetBackendPrivKey()
	require.Nil(t, err)
	walletAddress := []byte("0xf2e3878c9ab6a377d331e252f6bf3673d8e87323")

	sig, err := ConstructAndSignClaim(privKey, walletAddress, []byte(uuid), 10000, vat)
	require.Nil(t, err)

	expectedSig, _ := hex.DecodeString("0bbb76f330fe36625b3c932055b5e7b5a7adb86b1e19c727cf21f8ada45299a97d35232bbc3205663b610ae2f3e2017eecc6ad62f7b22afa846762d666bb6ec81b")

	assert.Equal(t, expectedSig, sig)
}

func Test_ConstructClaimAndVerify(t *testing.T) {
	uuid := "d18ac3989ae74da398c8ab26de41bb7c"
	signerAddress := "0x7baD7944E5d43CD50c14F4D159e706a68E8Ffe09"
	walletAddress := []byte("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	sig, _ := hex.DecodeString("88c45b40c1522208c60120f5199d7d945fcf5fceec4b3846beba7c7abaabc13f60b503b4a8b7e4a6bdc4206645bd59bbdd3f8ebc450d98e70fb67a865e78d2191b")

	claim, err := constructClaim(walletAddress, []byte(uuid), 10000, vat)
	require.Nil(t, err)

	hash := crypto.Keccak256Hash(claim)
	ethSigner := crypto.Keccak256Hash([]byte("\x19Ethereum Signed Message:\n32"), hash.Bytes())
	if sig[64] >= 27 {
		sig[64] -= 27
	}

	recoveredPubKey, err := crypto.SigToPub(ethSigner.Bytes(), sig)
	require.Nil(t, err)
	recoveredAddress := crypto.PubkeyToAddress(*recoveredPubKey)
	require.Equal(t, signerAddress, recoveredAddress.String())

	pubKeyBytes := crypto.FromECDSAPub(recoveredPubKey)
	success := crypto.VerifySignature(pubKeyBytes, ethSigner.Bytes(), sig[:64])
	require.True(t, success)
}
