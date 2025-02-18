package service

import (
	"encoding/hex"
	"errors"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/crypto"
)

var (
	privKey []byte
	pubKey  []byte
)

func NewAuthService() {
	seedBytes, err := hex.DecodeString(config.Config.Jwt.KeySeedHex)
	if err != nil {
		panic(err)
	}

	sk := crypto.NewEdKey(seedBytes)
	privKey = sk
	pubKey = sk[32:]
}

func RefreshToken(token, refresh string) (string, string, error) {
	claims, err := crypto.GetClaims(token, config.Config.Jwt.Secret, false)
	if err != nil {
		return "", "", errors.New("error while retrieving claims: " + err.Error())
	}

	refreshBytes, err := hex.DecodeString(refresh)
	if err != nil {
		return "", "", errors.New("error while decoding refresh token: " + err.Error())
	}

	err = crypto.VerifySignature(pubKey, []byte(token), refreshBytes)
	if err != nil {
		return "", "", errors.New("error while verifying signature: " + err.Error())
	}

	newJwtObj, err := newJwt(claims.Address)
	if err != nil {
		return "", "", errors.New("error while creating new jwt: " + err.Error())
	}

	newRefresh, err := crypto.SignPayload(privKey, []byte(newJwtObj))
	if err != nil {
		return "", "", errors.New("error while signing peyload: " + err.Error())
	}

	return newJwtObj, hex.EncodeToString(newRefresh), nil
}

func MakeJwtAndRefresh(address string) (string, string, error) {
	jwt, err := newJwt(address)
	if err != nil {
		return "", "", errors.New("error while creating new jwt: " + err.Error())
	}

	refresh, err := crypto.SignPayload(privKey, []byte(jwt))
	if err != nil {
		return "", "", errors.New("error while signing payload: " + err.Error())
	}

	return jwt, hex.EncodeToString(refresh), nil
}

func newJwt(address string) (string, error) {
	return crypto.GenerateJwt(
		address,
		config.Config.Jwt.Secret,
		config.Config.Jwt.Issuer,
		config.Config.Jwt.ExpiryMins,
	)
}
