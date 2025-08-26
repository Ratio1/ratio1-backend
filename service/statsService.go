package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func DailyGetStats() {
	oldStats, err := storage.GetLatestStats()
	if err != nil {
		fmt.Println("error getting latest stats: " + err.Error())
		return
	} else if oldStats == nil {
		oldStats = &model.Stats{
			TotalTokenBurn:           big.NewInt(0),
			TotalNdContractTokenBurn: big.NewInt(0),
			TotalMinted:              big.NewInt(0),
			TotalPOAIRewards:         big.NewInt(0),
			LastBlockNumber:          0,
		}
	}

	cspAddresses, err := getAllCSPAddress() // map[cspAddress]ownerAddress
	if err != nil {
		fmt.Println("Error while retrieving csp addresses: " + err.Error())
		return
	}

	from := oldStats.LastBlockNumber
	to, err := getChainLastBlockNumber()
	if err != nil {
		fmt.Println("error getting last block number: " + err.Error())
		return
	}

	/*Fetch all allocation events and create allocations on db*/
	allocEvents, err := fetchAllocationEvents(cspAddresses, from, to)
	if err != nil {
		fmt.Println("Error fetching events: " + err.Error())
		return
	}

	time.Sleep(1 * time.Second) // to avoid "429 Too Many Requests" error from infura

	/* TODO use for poai
	err = generateAllocations(allocEvents)
	if err != nil {
		fmt.Println("Error generating allocations: " + err.Error())
		return
	}
	*/

	dailyPoaiReward := big.NewInt(0)
	for _, e := range allocEvents {
		dailyPoaiReward = dailyPoaiReward.Add(dailyPoaiReward, &e.UsdcAmountPayed)
	}

	dailyMinted, err := getPeriodMintedAmount(from, to)
	if err != nil {
		fmt.Println("error getting daily minted: " + err.Error())
		return
	}

	time.Sleep(1 * time.Second) // to avoid "429 Too Many Requests" error from infura

	dailyTokenBurn, err := getPeriodBurnedAmount(from, to)
	if err != nil {
		fmt.Println("error getting daily token burn: " + err.Error())
		return
	}

	dailyNdContractTokenBurn, err := getPeriodNdContractBurnedAmount(from, to)
	if err != nil {
		fmt.Println("error getting daily nd contract token burn: " + err.Error())
		return
	}

	time.Sleep(1 * time.Second) // to avoid "429 Too Many Requests" error from infura

	totalSupply, err := getTotalSupply()
	if err != nil {
		fmt.Println("error getting total supply: " + err.Error())
		return
	}

	teamWalletsSupply, err := getTeamWalletsSupply()
	if err != nil {
		fmt.Println("error getting team wallets supply: " + err.Error())
		return
	}

	time.Sleep(1 * time.Second) // to avoid "429 Too Many Requests" error from infura

	dailyUsdcLocked, err := getDailyUsdcLocked(cspAddresses)
	if err != nil {
		fmt.Println("error getting daily USDC locked: " + err.Error())
		return
	}

	dailyActiveJobs, err := getDailyActiveJobs()
	if err != nil {
		fmt.Println("error getting daily active jobs: " + err.Error())
		return
	}

	stats := model.Stats{
		CreationTimestamp:        time.Now().UTC(),
		DailyActiveJobs:          dailyActiveJobs,
		DailyUsdcLocked:          dailyUsdcLocked,
		DailyTokenBurn:           dailyTokenBurn,
		DailyNdContractTokenBurn: dailyNdContractTokenBurn,
		DailyMinted:              dailyMinted,
		DailyPOAIRewards:         dailyPoaiReward,
		TotalSupply:              totalSupply,
		TeamWalletsSupply:        teamWalletsSupply,
		TotalTokenBurn:           big.NewInt(0).Add(oldStats.TotalTokenBurn, dailyTokenBurn),
		TotalNdContractTokenBurn: big.NewInt(0).Add(oldStats.TotalNdContractTokenBurn, dailyNdContractTokenBurn),
		TotalMinted:              big.NewInt(0).Add(oldStats.TotalMinted, dailyMinted),
		TotalPOAIRewards:         big.NewInt(0).Add(oldStats.TotalPOAIRewards, dailyPoaiReward),
		LastBlockNumber:          to,
	}

	err = storage.CreateStats(&stats)
	if err != nil {
		fmt.Println("error storing daily stats: " + err.Error())
		return
	}
}

func getChainLastBlockNumber() (int64, error) {
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client")
	}
	defer client.Close()

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, errors.New("error while retrieving header from client: " + err.Error())
	}

	return header.Number.Int64(), nil
}

func getDailyUsdcLocked(cspAddresses map[string]string) (*big.Int, error) {
	tokenAddress := common.HexToAddress(config.Config.USDCContractAddress)

	const erc20ABI = `[{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"},
	{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return big.NewInt(0), errors.New("error while parsing abi: " + err.Error())
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return big.NewInt(0), errors.New("error while dialing client")
	}
	defer client.Close()

	var totalCspContractBalance = big.NewInt(0)

	for addrStr := range cspAddresses {
		teamAddress := common.HexToAddress(addrStr)

		// Pack balanceOf call
		balanceData, err := parsedABI.Pack("balanceOf", teamAddress)
		if err != nil {
			return big.NewInt(0), errors.New("error packing balanceOf: " + err.Error())
		}

		msg := ethereum.CallMsg{
			To:   &tokenAddress,
			Data: balanceData,
		}

		result, err := client.CallContract(context.Background(), msg, nil)
		if err != nil {
			return big.NewInt(0), errors.New("error calling balanceOf for " + addrStr)
		}

		var balance *big.Int
		err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
		if err != nil {
			return big.NewInt(0), errors.New("error unpacking balanceOf for " + addrStr + ": " + err.Error())
		}

		totalCspContractBalance = totalCspContractBalance.Add(totalCspContractBalance, balance)
	}
	return totalCspContractBalance, nil
}

func getDailyActiveJobs() (int, error) {
	tokenAddress := common.HexToAddress(config.Config.PoaiManagerAddress)

	const poaiManagerAbi = `[{
      "inputs": [],
      "name": "nextJobId",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    }
]`

	parsedABI, err := abi.JSON(strings.NewReader(poaiManagerAbi))
	if err != nil {
		return 0, errors.New("error while parsing abi: " + err.Error())
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return 0, errors.New("error while dialing client: " + err.Error())
	}
	defer client.Close()

	jobIdPack, err := parsedABI.Pack("nextJobId")
	if err != nil {
		return 0, errors.New("error packing jobId: " + err.Error())
	}

	msg := ethereum.CallMsg{
		To:   &tokenAddress,
		Data: jobIdPack,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return 0, errors.New("error calling jobId: " + err.Error())
	}

	var activeJobsAsBigInt *big.Int
	err = parsedABI.UnpackIntoInterface(&activeJobsAsBigInt, "nextJobId", result)
	if err != nil {
		return 0, errors.New("error unpacking jobId: " + err.Error())
	}

	activeJobs := int(activeJobsAsBigInt.Int64())

	return activeJobs - 1, nil
}

func getAllCSPAddress() (map[string]string, error) { // map[cspAddress]ownerAddress
	contractAddress := common.HexToAddress(config.Config.PoaiManagerAddress)
	const cspEscrow = `[{
      "inputs": [],
      "name": "getAllCspsWithOwner",
      "outputs": [
        {
          "components": [
            {
              "internalType": "address",
              "name": "cspAddress",
              "type": "address"
            },
            {
              "internalType": "address",
              "name": "cspOwner",
              "type": "address"
            }
          ],
          "internalType": "struct CspWithOwner[]",
          "name": "",
          "type": "tuple[]"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    }

]`
	parsedABI, err := abi.JSON(strings.NewReader(cspEscrow))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	data, err := parsedABI.Pack("getAllCspsWithOwner")
	if err != nil {
		return nil, errors.New("error while packing interface: " + err.Error())
	}

	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return nil, errors.New("error while dialing client")
	}
	defer client.Close()

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, errors.New("error while calling contract")
	}

	addresses := []struct {
		CspAddress common.Address
		CspOwner   common.Address
	}{}

	err = parsedABI.UnpackIntoInterface(&addresses, "getAllCspsWithOwner", result)
	if err != nil {
		return nil, errors.New("error while unpacking interface: " + err.Error())
	}

	cspOwners := make(map[string]string)
	for _, a := range addresses {
		cspOwners[a.CspAddress.String()] = a.CspOwner.String()
	}

	return cspOwners, nil
}

func fetchAllocationEvents(cspOwners map[string]string, from, to int64) ([]model.Allocation, error) {
	var addresses []common.Address
	for k := range cspOwners {
		addresses = append(addresses, common.HexToAddress(k))
	}

	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	eventSignatureAsBytes := []byte(config.Config.AllocationEventSignature)
	eventHash := crypto.Keccak256Hash(eventSignatureAsBytes)

	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: addresses,
		Topics:    [][]common.Hash{{eventHash}},
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return nil, errors.New("error while dialing client: " + err.Error())
	}
	defer client.Close()

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		return nil, errors.New("error while filtering logs: " + err.Error())
	}

	var events []model.Allocation
	for _, vLog := range logs {
		event, err := decodeAllocLogs(vLog)
		if err != nil {
			fmt.Println("error while decoding logs: " + err.Error())
			continue
		}
		event.CspOwner = cspOwners[event.CspAddress]
		events = append(events, *event)
	}

	return events, nil
}

func decodeAllocLogs(vLog types.Log) (*model.Allocation, error) {
	parsedABI, err := abi.JSON(strings.NewReader(config.Config.AllocLogsAbi))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	event := struct {
		JobId       string
		NodeAddress common.Address
		NodeOwner   common.Address
		UsdcAmount  *big.Int
	}{}

	err = parsedABI.UnpackIntoInterface(&event, "RewardsAllocatedV2", vLog.Data)
	if err != nil {
		return nil, errors.New("error while unpacking interface: " + err.Error())
	}

	result := model.Allocation{
		CspAddress:  vLog.Address.String(),
		TxHash:      vLog.TxHash.Hex(),
		BlockNumber: int64(vLog.BlockNumber),

		NodeAddress:     event.NodeAddress.String(),
		UserAddress:     event.NodeOwner.String(),
		JobId:           event.JobId,
		UsdcAmountPayed: *event.UsdcAmount,
	}
	return &result, nil
}

/*
func generateAllocations(allocEevents []model.Allocation) error {
	for _, event := range allocEevents {
		err := storage.CreateAllocation(&event)
		if err != nil {
			return errors.New("error while saving allocation: " + err.Error())
		}
	}
	return nil
}*/
