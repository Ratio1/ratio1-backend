package crypto

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func VerifySafeSignature(safeAddress string, message string, signature string) error {
	messageHash := crypto.Keccak256Hash(
		[]byte("\x19Ethereum Signed Message:\n"+fmt.Sprintf("%d", len(message))),
		[]byte(message),
	).Bytes()

	safeAddr := common.HexToAddress(safeAddress)

	const abiJSON = `[{"constant":true,"inputs":[{"name":"_hash","type":"bytes32"},{"name":"_signature","type":"bytes"}],"name":"isValidSignature","outputs":[{"name":"magicValue","type":"bytes4"}],"payable":false,"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return err
	}

	sigBytes, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return err
	}

	if len(messageHash) != 32 {
		return errors.New("invalid hash length")
	}

	if len(sigBytes) != 65 {
		return errors.New("invalid signature length")
	}

	var hashArray [32]byte
	copy(hashArray[:], messageHash)

	input, err := parsedABI.Pack("isValidSignature", hashArray, sigBytes)
	if err != nil {
		return err
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return err
	}
	defer client.Close()

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
