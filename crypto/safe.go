package crypto

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func VerifySafeSignature(safeAddress string, message string, signature string) error {
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return err
	}
	defer client.Close()

	messageHash := crypto.Keccak256Hash([]byte(message)).Hex()
	safeAddr := common.HexToAddress(safeAddress)

	const abiJSON = `[{"constant":true,"inputs":[{"name":"_hash","type":"bytes32"},{"name":"_signature","type":"bytes"}],"name":"isValidSignature","outputs":[{"name":"magicValue","type":"bytes4"}],"payable":false,"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return err
	}

	hashBytes, err := hex.DecodeString(strings.TrimPrefix(messageHash, "0x"))
	if err != nil {
		return err
	}

	sigBytes, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return err
	}

	input, err := parsedABI.Pack("isValidSignature", hashBytes, sigBytes)
	if err != nil {
		return err
	}

	msg := ethereum.CallMsg{
		To:   &safeAddr,
		Data: input,
	}

	output, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return err
	}

	// 0x1626ba7e is the magic value for valid signatures on EIP-1271
	expected := [4]byte{0x16, 0x26, 0xba, 0x7e}
	var actual [4]byte
	copy(actual[:], output[:4])

	if actual != expected {
		return errors.New("invalid signature")
	}

	return nil
}
