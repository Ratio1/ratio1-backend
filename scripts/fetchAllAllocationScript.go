package main

import (
	"crypto/ecdsa"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

/*
MAKE sure to set all the needed variabels berfore running
*/

/*
func main() {
	var err error
	sk, err = loadPrivateKeyFromPemFile(PrivateKeyPath)
	if err != nil {
		fmt.Println("cannot load private key from file: " + err.Error())
		return
	}

	fmt.Println("beginning of all allocatoin script")
	allocations := GetAllAllocations()
	if allocations == nil {
		return
	}

	fmt.Println("lenght of allocation: ", len(allocations))

	fmt.Println("saving data to json")
	data, err := json.Marshal(allocations)
	if err != nil {
		fmt.Println(err.Error())
	}

	file, err := os.Create("allocations.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		panic(err)
	}
}
*/

func GetAllAllocations() []model.Allocation {
	client, err := ethclient.Dial(InfuraApiUrl + InfuraSecret)
	if err != nil {
		fmt.Println("error while dialing client:", err)
		return nil
	}
	defer client.Close()

	latestBlock, err := getChainLastBlockNumber(client)
	if err != nil {
		fmt.Println("error getting last block number:", err)
		return nil
	}

	cspAddresses, err := getAllCSPAddress(client)
	if err != nil {
		fmt.Println("Error while retrieving csp addresses:", err)
		return nil
	}

	// Get all new allocation to create them on DB and fetch all nodeAddre-nodeOwner for old event (missing match)
	newAllocEvent, err := fetchAllocationEvents(cspAddresses, 0, latestBlock, client)
	if err != nil {
		fmt.Println("Error fetching allocation events:", err)
		return nil
	}

	fmt.Println("lenght of newAllocEvent: ", len(newAllocEvent))

	newData, _ := json.Marshal(newAllocEvent)
	_ = os.WriteFile("new.json", newData, 0644)

	nodeOwner := make(map[string]string) //map[nodeAddress]userAddress
	for _, alloc := range newAllocEvent {
		nodeOwner[alloc.NodeAddress] = alloc.UserAddress
	}

	//Get all old allocation events and add mathcing nodeAddress-NodeOwner if present
	oldAllocEvent, err := fetchOldAllocationEvents(cspAddresses, 0, latestBlock, client)
	if err != nil {
		fmt.Println("Error fetching allocation events:", err)
		return nil
	}

	fmt.Println("lenght of oldAllocEvent: ", len(oldAllocEvent))

	for i, alloc := range oldAllocEvent {
		if v, ok := nodeOwner[alloc.NodeAddress]; ok {
			alloc.UserAddress = v
			oldAllocEvent[i].UserAddress = v
		} else {
			fmt.Println("no user address for node address: " + alloc.NodeAddress)
			continue
		}
		newAllocEvent = append(newAllocEvent, alloc)
	}

	allJobsId := make(map[string]*Response)
	for _, a := range newAllocEvent {
		allJobsId[a.JobId] = nil
	}

	for k := range allJobsId {
		res, err := getJobDetails(k, "https://deeploy-api.ratio1.ai/get_oracle_job_details")
		if err != nil {
			continue
		}
		allJobsId[k] = res
	}

	for i, a := range newAllocEvent {
		if v := allJobsId[a.JobId]; v != nil {
			a.JobName = v.Result.JobName
			a.JobType = strconv.Itoa(v.Result.JobType)
			a.ProjectName = v.Result.ProjectName
			newAllocEvent[i] = a
		}
	}

	olddata, _ := json.Marshal(oldAllocEvent)
	_ = os.WriteFile("old.json", olddata, 0644)

	return newAllocEvent
}

type Response struct {
	Result Result `json:"result"`
}
type Result struct {
	JobName     string `json:"job_name"`
	JobType     int    `json:"job_type"`
	ProjectName string `json:"project_name"`
}

func getJobDetails(jobId, api string) (*Response, error) {
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
	message := "Please sign this message for Deeploy: "
	messageByte := []byte(message)
	nodeAddress := crypto.PubkeyToAddress(sk.PublicKey).String()
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
	sig, err := crypto.Sign(ethSigner.Bytes(), sk)
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

func loadPrivateKeyFromPemFile(filepath string) (*ecdsa.PrivateKey, error) {
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
