package service

import (
	"crypto/ecdsa"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"os"
	"sync"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/crypto"
)

var _sk *ecdsa.PrivateKey
var skMutex sync.Mutex

func GetAddress() (string, error) {
	sk, err := GetBackendPrivKey()
	if err != nil {
		return "", err
	}

	return crypto.PubkeyToAddress(sk.PublicKey).String(), nil
}

func GetBackendPrivKey() (*ecdsa.PrivateKey, error) {
	skMutex.Lock()
	defer skMutex.Unlock()
	if _sk == nil {
		privKeyPath := os.Getenv("NAEURAL_PEM_FILE")
		if privKeyPath == "" {
			return nil, errors.New("NAEURAL_PEM_FILE is not set")
		}

		sk, err := LoadPrivateKeyFromPemFile(privKeyPath)
		if err != nil {
			return nil, errors.New("cannot load private key from file: " + err.Error())
		}

		_sk = sk
	}

	return _sk, nil
}

type pkcs8Key struct {
	Version             int
	PrivateKeyAlgorithm pkix.AlgorithmIdentifier
	PrivateKey          []byte
}

type ecPrivateKey struct {
	Version       int
	PrivateKey    []byte
	NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
	PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
}

func LoadPrivateKeyFromPemFile(filepath string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, errors.New("failed to read file: " + err.Error())
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("failed to decode PEM block")
	}

	var pkcs8 pkcs8Key
	if _, err := asn1.Unmarshal(block.Bytes, &pkcs8); err != nil {
		return nil, errors.New("cannot unmarshal PKCS8: " + err.Error())
	}
	var ecKey ecPrivateKey
	if _, err := asn1.Unmarshal(pkcs8.PrivateKey, &ecKey); err != nil {
		return nil, errors.New("cannot unmarshal EC private key: " + err.Error())
	}

	priv, _ := btcec.PrivKeyFromBytes(ecKey.PrivateKey)
	return priv.ToECDSA(), nil
}
