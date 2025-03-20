package service

import (
	libed25519 "crypto/ed25519"
	"encoding/hex"
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/crypto"
	"github.com/stretchr/testify/require"
)

func Test_CreateAndRefreshBeforeExpireShouldNotWork(t *testing.T) {
	seed := "202d2274940909b4f3c23691c857d7d3352a0574cfb96efbf1ef90cbc66e2cbc"
	msg := []byte("all your tokens")

	seedBytes, _ := hex.DecodeString(seed)

	sk := crypto.NewEdKey(seedBytes)
	pk := sk[libed25519.PublicKeySize:]

	sig, _ := crypto.SignPayload(sk, msg)
	verifyErr := crypto.VerifySignature(pk, msg, sig)
	require.Nil(t, verifyErr)

	config.Config.Jwt = config.JwtConfig{
		Secret:     "bitcoin-to-1-milly",
		Issuer:     "localhost:5000",
		KeySeedHex: "d6592724167553acf9c8cba9a7dbc7f514efc757d7906546cecfdfc5d4c2e8d1",
		ExpiryMins: -1,
	}
	NewAuthService()

	jwt, refresh, err := MakeJwtAndRefresh(string(pk.Seed()))
	require.Nil(t, err)

	_, _, err = RefreshToken(jwt, refresh)
	require.Nil(t, err)
}

func Test_Erd_CreateAndRefreshAfterExpireShouldWork(t *testing.T) {
	seed := "202d2274940909b4f3c23691c857d7d3352a0574cfb96efbf1ef90cbc66e2cbc"
	msg := []byte("all your tokens")

	seedBytes, _ := hex.DecodeString(seed)

	sk := crypto.NewEdKey(seedBytes)
	pk := sk[libed25519.PublicKeySize:]

	sig, _ := crypto.SignPayload(sk, msg)
	verifyErr := crypto.VerifySignature(pk, msg, sig)
	require.Nil(t, verifyErr)

	config.Config.Jwt = config.JwtConfig{
		Secret:     "bitcoin-to-1-milly",
		Issuer:     "127.0.0.1:5000",
		KeySeedHex: "d6592724167553acf9c8cba9a7dbc7f514efc757d7906546cecfdfc5d4c2e8d1",
		ExpiryMins: -1,
	}
	NewAuthService()

	jwt, refresh, err := MakeJwtAndRefresh(string(pk.Seed()))
	require.Nil(t, err)

	// Should succeed because token expired.
	jwt, refresh, err = RefreshToken(jwt, refresh)
	require.Nil(t, err)

	jwt, refresh, err = RefreshToken(jwt, refresh)
	require.Nil(t, err)

	jwt, refresh, err = RefreshToken(jwt, refresh)
	require.Nil(t, err)

	_, _, err = RefreshToken(jwt, refresh)
	require.Nil(t, err)
}
