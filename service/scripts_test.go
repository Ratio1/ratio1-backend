package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	infuraURL       = "https://base-sepolia.infura.io/v3/533c2b6ac99b4f11b513d25cfb5dffd1"
	contractAddress = "0x9aB4e425c7dFFC7Aa1A7a262727b0b663e047571"
)

const contractiNodeActiveABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "nodeAddress",
				"type": "address"
			}
		],
		"name": "isNodeActive",
		"outputs": [
			{
				"internalType": "bool",
				"name": "",
				"type": "bool"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

const contractSignersABI = `[
	{
		"inputs": [],
		"name": "getSigners",
		"outputs": [
			{
				"internalType": "address[]",
				"name": "",
				"type": "address[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var AddressArray = []string{
	"0x99Ac885B00a150cFc93EA1A51FcC035C17aCB02c",
	"0x49943d72CF93B69AE22c7194093804635a99eF2B",
	"0x46b0e749d1A7f09Bff2e58324DA5ECa093591371",
	"0x46d0138687d11519E4aA36d33679FD8392275266",
	"0xAE9a7bc5164D02A76603EDDE0D519575db265912",
	"0x338f52adDce9811825A90757aB3D6CE89469081E",
	"0x64A4C148FDa4a0D900daB9417cd65968993d30b3",
	"0x4B50d4ac46c3ba0F463603587d41c67213A0a091",
	"0x78361D7eA67a93E7d0C001b75178336F3cC8985e",
	"0xA277147dC88cf8D81e45a26d0481AAF09655e842",
	"0xD090c279ED45c9Ec2F45370e7d0fC1629c0E991E",
	"0x0351B8AEB7fC6710e23D6317aA606ac5e17ae023",
	"0xc3a8cCD486DdaFa651F4a3D4f10209e05BAbf18e",
	"0x1Fe3222f6a2844364E2BDc796e0Df547ea26B815",
	"0x1351504af17BFdb80491D9223d6Bcb6BB964DCeD",
	"0x3a663E677c1A89D1B83B48F4763FDbc03e9E139D",
	"0x4caBC3A71705dD3C6c9cCF850ae2E438f23b90e5",
	"0xDbDc18Db87D6242A483cAB9302e747a158C8AbD9",
	"0x87fa64D9Ce3229b2Dfc281563814f7A5B1108066",
	"0x49C1704AfF66546769cdc037cdb5F9338EbfD457",
	"0xCF1C4BeaDa5bb0A2195633Ad6CFD945ec6aA737D",
	"0x53469deFa4eb7f076331b04246c7859572200fBf",
	"0xC98E989C5b32C10952c052B7e86FcE0f17dcEcfF",
	"0x62Ce480D9C9CA2B1E756a47Ef182f09E95407dAe",
	"0x81920D966C44676267205D8aAa9324E39DA0D9eC",
	"0x8A35B74f242BD112E46e12dd93E5d3F71DB1cB94",
	"0x35415FF0B7115C40b25E5d0A358E806c2A231D50",
	"0x3E12f2e75F0C4b4C1c1d33488da9DeF1EC20A8d1",
	"0x787B18366b5AE8380cc720DD4066aC161EE83232",
	"0x8C597C8B51e117360A0f9088865dcAf20B796Db2",
	"0xDcB068dfE65cDcF86A0F035E78350d99Cb22c4f4",
	"0x817F5e5245aeD1Ccd66C12199A1c200b68f341de",
	"0x58d9361CFAB7E1169AD6391f2EF27fef040b7b4D",
	"0x1ee2dA9c049f744947c7C098aE99bE0a2654eE65",
	"0xe1170B4139497612Ef35852b9055e0d819e128Cc",
	"0xE486F0d594e9F26931fC10c29E6409AEBb7b5144",
	"0x93B04EF1152D81A0847C2272860a8a5C70280E14",
	"0x49CD9D9528A4F6aEf94A0EB63E7745Eca4F9b57e",
	"0x129a21A78EBBA79aE78B8f11d5B57102950c1Fc0",
	"0x2539fDD57f93b267E58d5f2E6F77063C0230F6F4",
	"0x7C07758C23DF14c2fF4b016F0ad58F2D4aF329a7",
	"0x369C7dfc6484528A472897Cae6A98EB05c49c122",
	"0x3f433eE77A6396b2Be8D51682ea66278F31CeF12",
	"0xe240d9cf8893d6bE9fb3Ac4C9CE1E504343b64a0",
	"0xb0c73Bf8F859D4b3DeFB28CF5AF33adC151B0Be7",
	"0x438E57B8f92D953d07ce41BcF9249AffDFCB9F98",
	"0x3bb330fe4BF4E5d45D901c40F9Eb9e3e68d6C744",
}

func Test_getvalidNodes(t *testing.T) {
	client, err := ethclient.Dial(infuraURL)
	if err != nil {
		log.Error("Failed to connect to Ethereum client: %v", err)
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(contractiNodeActiveABI))
	if err != nil {
		log.Error("Failed to parse ABI: %v", err)
	}

	contractAddr := common.HexToAddress(contractAddress)

	var validAddresses []string

	for _, addrStr := range AddressArray {
		nodeAddr := common.HexToAddress(addrStr)

		data, err := parsedABI.Pack("isNodeActive", nodeAddr)
		if err != nil {
			log.Error("Error packing call data for address %s: %v", addrStr, err)
			continue
		}

		msg := ethereum.CallMsg{
			To:   &contractAddr,
			Data: data,
		}

		result, err := client.CallContract(context.Background(), msg, nil)
		if err != nil {
			log.Error("Error calling contract for address %s: %v", addrStr, err)
			continue
		}

		var isActive bool
		if err := parsedABI.UnpackIntoInterface(&isActive, "isNodeActive", result); err != nil {
			log.Error("Error unpacking result for address %s: %v", addrStr, err)
			continue
		}

		if isActive {
			validAddresses = append(validAddresses, addrStr)
		}
	}

	jsonData, err := json.MarshalIndent(validAddresses, "", "  ")
	if err != nil {
		log.Error("Error marshaling JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}

func Test_getSigners(t *testing.T) {
	client, err := ethclient.Dial(infuraURL)
	if err != nil {
		log.Error("Failed to connect to Ethereum client: %v", err)
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(contractSignersABI))
	if err != nil {
		log.Error("Failed to parse ABI: %v", err)
	}

	contractAddr := common.HexToAddress(contractAddress)

	data, err := parsedABI.Pack("getSigners")
	if err != nil {
		log.Error("Errore nel packing dei dati per la funzione getSigners: %v", err)
	}

	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		log.Error("Errore nella chiamata al contratto: %v", err)
	}

	var signers []common.Address
	if err := parsedABI.UnpackIntoInterface(&signers, "getSigners", result); err != nil {
		log.Error("Errore nell'unpacking dei dati: %v", err)
	}

	fmt.Println("addresses = []string{")
	for _, signer := range signers {
		fmt.Printf("    \"%s\",\n", signer.Hex())
	}
}
