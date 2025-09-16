package service

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/ethereum/go-ethereum/crypto"
)

type Response struct {
	Result Result `json:"result"`
}
type Result struct {
	JobName     string `json:"job_name"`
	JobType     int    `json:"job_type"`
	ProjectName string `json:"project_name"`
}

func GetJobDetails(jobId, api string) (*Response, error) {
	nonce := "0x" + strconv.FormatInt(time.Now().UTC().UnixMilli(), 16)
	jobIdAsInt, err := strconv.Atoi(jobId)
	if err != nil {
		return nil, errors.New("error while parsing jobId: " + err.Error())
	}

	request, err := createRequest(jobIdAsInt, nonce)
	if err != nil {
		return nil, errors.New("error while creating request: " + err.Error())
	}

	var resp Response
	err = process.HttpPost(api, request, &resp)
	if err != nil {
		return nil, errors.New("error while retriveing job details: " + err.Error())
	}

	return &resp, nil
}

func createRequest(jobId int, nonce string) (any, error) {
	privKey, err := GetBackendPrivKey()
	if err != nil {
		return nil, errors.New("error while retrieving private key: " + err.Error())
	}

	message := "Please sign this message for Deeploy: "
	messageByte := []byte(message)
	nodeAddress := crypto.PubkeyToAddress(privKey.PublicKey).String()
	signableRequest := struct {
		JobId int    `json:"job_id"`
		Nonce string `json:"nonce"`
	}{
		JobId: jobId,
		Nonce: nonce,
	}
	data, err := json.MarshalIndent(signableRequest, "", " ")
	if err != nil {
		return nil, errors.New("error while doing json marshal on signable request: " + err.Error())
	}
	messageByte = append(messageByte, data...)

	jsonString := string(messageByte)
	jsonString = strings.ReplaceAll(jsonString, `": `, `":`)
	messageByte = []byte(jsonString)
	hash := crypto.Keccak256Hash(messageByte)
	ethSigner := crypto.Keccak256Hash([]byte("\x19Ethereum Signed Message:\n32"), hash.Bytes())
	sig, err := crypto.Sign(ethSigner.Bytes(), privKey)
	if err != nil {
		return nil, errors.New("error while signing payload: " + err.Error())
	}

	return struct {
		Request any `json:"request"`
	}{Request: struct {
		JobId         int    `json:"job_id"`
		Nonce         string `json:"nonce"`
		NodeAddress   string `json:"EE_ETH_SENDER"`
		NodeSignature string `json:"EE_ETH_SIGN"`
	}{
		JobId:         jobId,
		Nonce:         nonce,
		NodeAddress:   nodeAddress,
		NodeSignature: hex.EncodeToString(sig),
	}}, nil
}
