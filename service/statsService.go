package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/ratio1abi"
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

	if getEpoch(oldStats.CreationTimestamp) == getEpoch(time.Now()) { //get epoch of r1 mainnet, if today is already present, skip.
		fmt.Println("stats already fetched")
		return
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

	/*Fetch all allocation events*/
	allocEvents, err := fetchAllocationEvents(cspAddresses, from, to)
	if err != nil {
		fmt.Println("Error fetching events: " + err.Error())
		return
	}

	if len(allocEvents) == 0 {
		fmt.Println("No events fetched, allocation hasn't occured yet")
		return
	}

	time.Sleep(1 * time.Second)

	/* get all unique nodes and fetch their owner */
	nodeToOwner := make(map[string]string) // map[nodeAddress]ownerAddress
	for _, a := range allocEvents {
		nodeToOwner[a.NodeAddress] = ""
	}
	uniqueNodes := make([]string, 0, len(nodeToOwner))
	for nodeAddr := range nodeToOwner {
		uniqueNodes = append(uniqueNodes, nodeAddr)
	}

	nodeToOwner, err = getNodeOwners(uniqueNodes)
	if err != nil {
		fmt.Println("Error fetching node owners: " + err.Error())
		return
	}

	for i, a := range allocEvents {
		if owner, ok := nodeToOwner[a.NodeAddress]; ok {
			allocEvents[i].UserAddress = owner
		}
	}

	/*Fetch all burned events */
	burnEvents, err := fetchBurnEvents(cspAddresses, from, to)
	if err != nil {
		fmt.Println("Error fetching events: " + err.Error())
		return
	} //if allocation has happened, burn has happened too, so no need to check len(burnEvents)==0

	time.Sleep(1 * time.Second)

	/* get all blocks timestamp(burned event happen same time as allocation)*/
	blocks := make(map[int64]*time.Time)
	for _, a := range allocEvents {
		blocks[a.BlockNumber] = nil
	}

	for k := range blocks {
		v, err := getBlockTimestamp(k)
		if err != nil {
			fmt.Println("cannot fetch correct timestamp")
			return
		}
		blocks[k] = &v
		time.Sleep(1 * time.Second)
	}

	/* get all jobs details */
	allJobsId := make(map[string]*Response)
	for _, a := range allocEvents {
		allJobsId[a.JobId] = nil
	}

	for k := range allJobsId {
		res, err := GetJobDetails(k, config.Config.DeeployApi)
		if err != nil {
			continue
		}
		allJobsId[k] = res
	}

	/* in each allocation, add timestamp and job details */
	for i, a := range allocEvents {
		if v := blocks[a.BlockNumber]; v != nil {
			a.AllocationCreation = *v
		}
		if v := allJobsId[a.JobId]; v != nil {
			a.JobName = v.Result.JobName
			a.JobType = model.JobType(v.Result.JobType)
			a.ProjectName = v.Result.ProjectName
		}
		allocEvents[i] = a
	}

	/* get all currency*/
	currencyMap, err := GetFreeCurrencyValues() //map[USD,EUR...]ratio always based 1 usd -> value
	if err != nil {
		fmt.Println("could not fetch currency map: ", err.Error())
		return
	}

	/* get preferences for eache csp owner*/
	cspPreferences := make(map[string]*model.Preference) // map[cspOwnerAddress]Preference
	for _, v := range cspAddresses {
		preference, err := storage.GetPreferenceByAddress(v)
		if err != nil || preference == nil {
			preference = &model.Preference{
				LocalCurrency: "USD",
			}
		}
		cspPreferences[v] = preference
	}

	/* in each burn, add timestamp + exchange ratio and preferred currency*/
	for i, b := range burnEvents {
		if v := blocks[b.BlockNumber]; v != nil {
			b.BurnTimestamp = *v
		}
		if pref, ok := cspPreferences[b.CspOwner]; ok && pref != nil {
			b.LocalCurrency = pref.LocalCurrency
			if ratio, ok := currencyMap[pref.LocalCurrency]; ok {
				b.ExchangeRatio = ratio
			}
		}
		burnEvents[i] = b
	}

	/* store all allocation events */
	err = generateAllocations(allocEvents)
	if err != nil {
		fmt.Println("Error generating allocations: " + err.Error())
		return
	}

	/* store all burn events */
	err = generateBurns(burnEvents)
	if err != nil {
		fmt.Println("Error generating burns: " + err.Error())
		return
	}

	/* calculate daily stats */
	dailyPoaiReward := big.NewInt(0)
	for _, e := range allocEvents {
		dailyPoaiReward.Add(dailyPoaiReward, e.GetUsdcAmountPayed()) //no need to assign to dailyPoaiReward
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

	dailyUsdcLocked, err := getDailyUsdcLocked()
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

func getDailyUsdcLocked() (*big.Int, error) {
	managerAddress := common.HexToAddress(config.Config.PoaiManagerAddress)
	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.PoaiManagerTotalBalanceAbi))
	if err != nil {
		return big.NewInt(0), errors.New("error while parsing abi: " + err.Error())
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return big.NewInt(0), errors.New("error while dialing client")
	}
	defer client.Close()

	// Pack getTotalEscrowsBalance call
	balanceData, err := parsedABI.Pack("getTotalEscrowsBalance")
	if err != nil {
		return big.NewInt(0), errors.New("error packing getTotalEscrowsBalance: " + err.Error())
	}

	msg := ethereum.CallMsg{
		To:   &managerAddress,
		Data: balanceData,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return big.NewInt(0), errors.New("error calling getTotalEscrowsBalance: " + err.Error())
	}

	var totalBalance *big.Int
	err = parsedABI.UnpackIntoInterface(&totalBalance, "getTotalEscrowsBalance", result)
	if err != nil {
		return big.NewInt(0), errors.New("error unpacking getTotalEscrowsBalance: " + err.Error())
	}

	return totalBalance, nil
}

func getDailyActiveJobs() (int, error) {
	tokenAddress := common.HexToAddress(config.Config.PoaiManagerAddress)

	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.PoaiManagerNextJobIdAbi))
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
	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.PoaiManagerGetAllCspsWithOwnerAbi))
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

	eventSignatureAsBytes := []byte(ratio1abi.AllocationEventSignature)
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
	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.AllocationLogsAbi))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	event := struct {
		NodeAddress common.Address
		UsdcAmount  *big.Int
	}{}

	err = parsedABI.UnpackIntoInterface(&event, "RewardsAllocatedV3", vLog.Data)
	if err != nil {
		return nil, errors.New("error while unpacking interface: " + err.Error())
	}

	jobIDBig := new(big.Int).SetBytes(vLog.Topics[1].Bytes())
	var jobID uint64
	if jobIDBig.BitLen() > 64 {
		return nil, fmt.Errorf("jobId too large for uint64: %s", jobIDBig.String())
	}
	jobID = jobIDBig.Uint64()
	result := model.Allocation{
		CspAddress:  vLog.Address.String(),
		TxHash:      vLog.TxHash.Hex(),
		BlockNumber: int64(vLog.BlockNumber),

		NodeAddress: event.NodeAddress.String(),
		JobId:       strconv.Itoa(int(jobID)),
	}
	result.SetUsdcAmountPayed(event.UsdcAmount)
	return &result, nil
}

func fetchBurnEvents(cspOwners map[string]string, from, to int64) ([]model.BurnEvent, error) {
	var addresses []common.Address
	for k := range cspOwners {
		addresses = append(addresses, common.HexToAddress(k))
	}

	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	eventSignatureAsBytes := []byte(ratio1abi.BurnEventSignature)
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

	var events []model.BurnEvent
	for _, vLog := range logs {
		event, err := decodeBurnLogs(vLog)
		if err != nil {
			fmt.Println("error while decoding logs: " + err.Error())
			continue
		}
		event.CspOwner = cspOwners[event.CspAddress]
		events = append(events, *event)
	}

	return events, nil
}

func decodeBurnLogs(vLog types.Log) (*model.BurnEvent, error) {
	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.BurnLogsAbi))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	event := struct {
		UsdcAmount *big.Int
		R1Amount   *big.Int
	}{}

	err = parsedABI.UnpackIntoInterface(&event, "TokensBurned", vLog.Data)
	if err != nil {
		return nil, errors.New("error while unpacking interface: " + err.Error())
	}

	result := model.BurnEvent{
		CspAddress:  vLog.Address.String(),
		TxHash:      vLog.TxHash.Hex(),
		BlockNumber: int64(vLog.BlockNumber),
	}
	result.SetUsdcAmountSwapped(event.UsdcAmount)
	result.SetR1AmountBurned(event.R1Amount)
	return &result, nil
}

func getBlockTimestamp(blockNumber int64) (time.Time, error) {
	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return time.Time{}, errors.New("error while dialing client")
	}
	defer client.Close()

	header, err := client.HeaderByNumber(context.Background(), big.NewInt(blockNumber))
	if err != nil {
		return time.Time{}, errors.New("error while retrieving block: " + err.Error())
	}

	return time.Unix(int64(header.Time), 0).UTC(), nil
}

func generateAllocations(allocEevents []model.Allocation) error {
	for _, event := range allocEevents {
		err := storage.CreateAllocation(&event)
		if err != nil {
			return errors.New("error while saving allocation: " + err.Error())
		}
	}
	return nil
}
func generateBurns(burnEvents []model.BurnEvent) error {
	for _, event := range burnEvents {
		err := storage.CreateBurnEvent(&event)
		if err != nil {
			return errors.New("error while saving Burn events: " + err.Error())
		}
	}
	return nil
}

/* This function is only available for mainnet*/
func getEpoch(date time.Time) int {
	mainnetStart := time.Unix(1748016000, 0)
	return int(date.Sub(mainnetStart) / (24 * time.Hour))
}

func getNodeOwners(nodes []string) (map[string]string, error) { // map[nodeAddress]nodeOwner
	contractAddress := common.HexToAddress(config.Config.ReaderAddress)
	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.ReaderNodeOwnersAbi))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	addrs := make([]common.Address, len(nodes))
	for i, s := range nodes {
		addrs[i] = common.HexToAddress(s)
	}

	data, err := parsedABI.Pack("getNdNodesOwners", addrs)
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
		NodeAddress common.Address
		Owner       common.Address
	}{}

	err = parsedABI.UnpackIntoInterface(&addresses, "getNdNodesOwners", result)
	if err != nil {
		return nil, errors.New("error while unpacking interface: " + err.Error())
	}

	nodeOwners := make(map[string]string)
	for _, a := range addresses {
		nodeOwners[a.NodeAddress.String()] = a.Owner.String()
	}

	return nodeOwners, nil
}
