package service

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/ethereum/go-ethereum/crypto"
)

func NewBuyLicenseTxTemplate(walletAddress, userUuid string, amount int) (string, int, error) {

	privKey, err := GetBackendPrivKey()
	if err != nil {
		return "", 0, errors.New("error while retrieving private key: " + err.Error())
	}

	sig, err := ConstructAndSignClaim(
		privKey,
		[]byte(walletAddress),
		[]byte(userUuid),
		amount,
	)

	if err != nil {
		return "", 0, errors.New("error while signing message: " + err.Error())
	}

	return hex.EncodeToString(sig), amount, nil
}

func ConstructAndSignClaim(privKey *ecdsa.PrivateKey, walletAddress, uuid []byte, amount int) ([]byte, error) {
	claim, err := constructClaim(walletAddress, uuid, amount)
	if err != nil {
		return nil, errors.New("error while constructing claims: " + err.Error())
	}
	hash := crypto.Keccak256Hash(claim)
	ethSigner := crypto.Keccak256Hash([]byte("\x19Ethereum Signed Message:\n32"), hash.Bytes())
	sig, err := crypto.Sign(ethSigner.Bytes(), privKey)
	if err != nil {
		return nil, errors.New("error while signing payload: " + err.Error())
	}
	/*
		In Solidity the 64th digit of a sign is the recovery digit
		it's required to be 27 or 28

		The crypto.sign function from eth library in go set 0 or 1, the std value from ECDSA
	*/
	if sig[64] < 27 {
		sig[64] += 27
	}

	return sig, nil
}

func constructClaim(walletAddress, uuid []byte, amount int) ([]byte, error) {
	addressBytes := walletAddress
	if len(walletAddress) == 42 && walletAddress[0] == '0' && walletAddress[1] == 'x' {
		addressBytes = walletAddress[2:] //remove "0x" if present
	}
	if len(addressBytes) != 40 {
		return nil, errors.New("address is not correct")
	}

	resultBytes, err := hex.DecodeString(string(addressBytes))
	if err != nil {
		return nil, errors.New("error while encoding address: " + err.Error())
	}

	resultBytes = append(resultBytes, padTo32Bytes(uuid)...)

	hexStr := strconv.FormatInt(int64(amount), 16)
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	hexBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, errors.New("error while encoding amount: " + err.Error())
	}

	resultBytes = append(resultBytes, padTo32Bytes(hexBytes)...)

	return resultBytes, nil
}

func padTo32Bytes(b []byte) []byte {
	if len(b) < 32 {
		// Pad the byte slice to the left with zeros
		padding := make([]byte, 32-len(b))
		return append(padding, b...)
	}
	return b
}
