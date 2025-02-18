package crypto

import (
	libed25519 "crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ElrondNetwork/elrond-go-crypto/signing"
	"github.com/ElrondNetwork/elrond-go-crypto/signing/ed25519"
	erdgoData "github.com/ElrondNetwork/elrond-sdk-erdgo/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSeed(t *testing.T) {
	key := generateSeed()
	t.Log(key)
}

func TestVerifyForCorrectSignature_ShouldPass(t *testing.T) {
	t.Parallel()
	pubKey := "XXnByecIEjQ8Ir/10T/YnCGWX6W48BW+fgmF+PP7iWQ="
	sig := "TJwsQOAbAw9n0twlVXJ2P7FmthrVWaIX5N7j5j6ebxPY0FgpTbRWm7TkbN1jepvQvAXQpsAp8ZLR5OseZnVjBQ=="

	pubKeyBytes, _ := base64.StdEncoding.DecodeString(pubKey)
	sigBytes, _ := base64.StdEncoding.DecodeString(sig)
	msg := []byte("msg")

	err := VerifySignature(pubKeyBytes, msg, sigBytes)
	assert.Nil(t, err)
}

func TestVerifyForIncorrectPubKeyLength_ShouldErr(t *testing.T) {
	t.Parallel()
	pubKey := "XXnByecIEjQ8Ir/10T/YnCGWX6W48BW+fgmF+PP7iWQQ=="
	sig := "TJwsQOAbAw9n0twlVXJ2P7FmthrVWaIX5N7j5j6ebxPY0FgpTbRWm7TkbN1jepvQvAXQpsAp8ZLR5OseZnVjBQ=="

	pubKeyBytes, _ := base64.StdEncoding.DecodeString(pubKey)
	sigBytes, _ := base64.StdEncoding.DecodeString(sig)
	msg := []byte("msg")

	err := VerifySignature(pubKeyBytes, msg, sigBytes)
	assert.Equal(t, ErrInvalidPublicKey, err)
}

func TestVerifyForIncorrectSig_ShouldErr(t *testing.T) {
	t.Parallel()
	pubKey := "XXnByecIEjQ8Ir/10T/YnCGWX6W48BW+fgmF+PP7iWQ="
	sig := "TJwsQOAbAw9n0twlVXJ2P7FmthrVWaIX5n7j5j6ebxPY0FgpTbRWm7TkbN1jepvQvAXQpsAp8ZLR5OseZnVjBQ=="

	pubKeyBytes, _ := base64.StdEncoding.DecodeString(pubKey)
	sigBytes, _ := base64.StdEncoding.DecodeString(sig)
	msg := []byte("msg")

	err := VerifySignature(pubKeyBytes, msg, sigBytes)
	assert.Equal(t, ErrInvalidSignature, err)
}

func TestNewEdKey_KeySignatureWillBeVerified(t *testing.T) {
	t.Parallel()

	seed := "202d2274940909b4f3c23691c857d7d3352a0574cfb96efbf1ef90cbc66e2cbc"
	msg := []byte("all your tokens are belong to us, kind ser")

	seedBytes, _ := hex.DecodeString(seed)

	sk := NewEdKey(seedBytes)
	pk := sk[libed25519.PublicKeySize:]

	sig, _ := SignPayload(sk, msg)

	verifyErr := VerifySignature(pk, msg, sig)
	require.Nil(t, verifyErr)
}

func Test_VerifyDevnetWalletGeneratedSignature(t *testing.T) {
	address, err := erdgoData.NewAddressFromBech32String("erd17s2pz8qrds6ake3qwheezgy48wzf7dr5nhdpuu2h4rr4mt5rt9ussj7xzh")
	require.Nil(t, err)

	message := []byte("cevaceva")
	sig, err := hex.DecodeString("8722fc7a40c84ab784d7cca3c94a334bd2da82fd55c827e242fe4bc3a7062342d7f61ac037bee380dac1237ea369bc390882059abb965ab98855139dc7745e0c")
	require.Nil(t, err)

	erdMsg := ComputeElrondSignableMessage(message)
	err = VerifySignature(address.AddressBytes(), erdMsg, sig)
	require.Nil(t, err)
}

func Test_ElrondGoCopyPasted(t *testing.T) {
	address, err := erdgoData.NewAddressFromBech32String("erd19pht2w242wcj0x9gq3us86dtjrrfe3wk8ffh5nhdemf0mce6hsmsupxzlq")
	require.Nil(t, err)

	message := []byte("test message")
	sig, err := hex.DecodeString("ec7a27cb4b23641ae62e3ea96d5858c8142e20d79a6e1710037d1c27b0d138d7452a98da93c036b2b47ee587d4cb4af6ae24c358f3f5f74f85580f45e072280b")
	require.Nil(t, err)

	erdMsg := ComputeElrondSignableMessage(message)
	err = VerifySignature(address.AddressBytes(), erdMsg, sig)
	require.Nil(t, err)
}

func Test_WebWalletRouteSignature(t *testing.T) {
	address, err := erdgoData.NewAddressFromBech32String("erd17s2pz8qrds6ake3qwheezgy48wzf7dr5nhdpuu2h4rr4mt5rt9ussj7xzh")
	require.Nil(t, err)

	message, err := hex.DecodeString("af8ffd30add45b0b7299497e41b3599c5acf81ce2e5989751950f4c25ec94581")
	require.Nil(t, err)

	sig, err := hex.DecodeString("96cb38a3b85fa0adcf2bba88c2453907323faceb2225869d99a42e1c9f65a8d822cb607723b23e5c62910122601ba0094ba043eec7a0c89ba4045e357fbee107")
	require.Nil(t, err)

	err = VerifySignature(address.AddressBytes(), message, sig)
	require.Nil(t, err)
}

/*
Pubkey: 5fda3897a7f79f0ff57321b5ab315ccaad22b112102f4b0c8dfdde29521c4853
Privkey: 0d57c7271eef44353ceba4c6721d1b62dd6c4e5c3c0624f7bcc25562896a4b0c5fda3897a7f79f0ff57321b5ab315ccaad22b112102f4b0c8dfdde29521c4853
Address bytes: f414111c036c35db662075f39120953b849f34749dda1e7157a8c75dae835979
00000032
f414111c036c35db662075f39120953b849f34749dda1e7157a8c75dae83597900000032
fd23b00948af24338c2fd59712fa30d09f5d1f8f97ff1dd40ecad211487c1c7f6c1c1d134c31b3d01093a155a272bfaf5372b2febb522c9f6e0da9243e2f4909
*/

func TestGenerate(t *testing.T) {
	suite := ed25519.NewEd25519()

	keyGenerator := signing.NewKeyGenerator(suite)
	priv, pub := keyGenerator.GeneratePair()

	pubByte, _ := pub.ToByteArray()
	privByte, _ := priv.ToByteArray()

	println("Pubkey: " + hex.EncodeToString(pubByte))
	println("Privkey: " + hex.EncodeToString(privByte))

	addressBech32 := "erd17s2pz8qrds6ake3qwheezgy48wzf7dr5nhdpuu2h4rr4mt5rt9ussj7xzh"
	address, err := erdgoData.NewAddressFromBech32String(addressBech32)
	require.Nil(t, err)

	println("Address bytes: " + hex.EncodeToString(address.AddressBytes()))

	tier := big.NewInt(50).Bytes()

	// Need to fill the bytes with 0 values
	if len(tier) != 4 {
		zeros := make([]byte, 4-len(tier))

		for i := 0; i < len(zeros); i++ {
			zeros[i] = 0
		}

		tier = append(zeros, tier...)
	}

	println(hex.EncodeToString(tier))

	claim := append(address.AddressBytes(), tier...)

	println(hex.EncodeToString(claim))

	sig := libed25519.Sign(privByte, claim)

	println(hex.EncodeToString(sig))

	success := libed25519.Verify(pubByte, claim, sig)
	require.True(t, success)
}

func Test_OnTheFly(t *testing.T) {
	pub, _ := hex.DecodeString("4b841614a2af0a6253ef482fa1b36038fec1778346f3afea086b76e840e797ca")
	msg, _ := hex.DecodeString("f414111c036c35db662075f39120953b849f34749dda1e7157a8c75dae83597900000032")
	sig, _ := hex.DecodeString("0979ce562d2eb70869aa01d2e81e00c1de70e1e98aac5a3cb2e81be5dfdb881a86fea051f730a593ccae5cbf959a2eeec170093ddf38c4af52ef6f56e73dbb0a")
	success := libed25519.Verify(pub, msg, sig)
	require.True(t, success)
}
