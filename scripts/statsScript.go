package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

/*
.####.##....##.########..#######.
..##..###...##.##.......##.....##
..##..####..##.##.......##.....##
..##..##.##.##.######...##.....##
..##..##..####.##.......##.....##
..##..##...###.##.......##.....##
.####.##....##.##........#######.
*/

/*
-Make sure to set the infuraSecret variable with your Infura project ID before running the script.
-Change sleepTime if you encounter rate limiting issues or if you have premium access. It can take up to 20 mins.
-Other parameters should not be changed unless you know what you're doing.
-Run the script using `go run scripts/statsScript.go` from base folder.

-At the end of execution, a file named stats.json will be created in the scripts folder.
*/

/*
.##.....##....###....########.
.##.....##...##.##...##.....##
.##.....##..##...##..##.....##
.##.....##.##.....##.########.
..##...##..#########.##...##..
...##.##...##.....##.##....##.
....###....##.....##.##.....##
*/

var (
	sleepTime = 1 * time.Second

	infuraSecret = ""
	infuraApiUrl = "https://base-mainnet.infura.io/v3/"

	allocationEventSignature = "RewardsAllocatedV2(uint256,address,address,uint256)"
	poaiManagerAddress       = "0xa8d7FFCE91a888872A9f5431B4Dd6c0c135055c1"
	usdcContractAddress      = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	ndContractAddress        = "0xE658DF6dA3FB5d4FBa562F1D5934bd0F9c6bd423"
	r1ContractAddress        = "0x6444C6c2D527D85EA97032da9A7504d6d1448ecF"
	teamAddresses            = []string{
		"0xABdaAC00E36007fB71b2059fc0E784690a991923",
		"0x9a7055e3FBA00F5D5231994B97f1c0216eE1C091",
		"0x745C01f91c59000E39585441a3F1900AeF72c5C1",
		"0x5d5F16f1848c87b49185A9136cdF042384e82BA8",
		"0x0A27F805Db42089d79B96A4133A93B2e5Ff1b28C",
	}
	allocLogsAbi = `[{
"anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "uint256",
          "name": "jobId",
          "type": "uint256"
        },
        {
          "indexed": false,
          "internalType": "address",
          "name": "nodeAddress",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "address",
          "name": "nodeOwner",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "usdcAmount",
          "type": "uint256"
        }
      ],
      "name": "RewardsAllocatedV2",
      "type": "event"
    }
]`
)

/*
.##.....##....###....####.##....##....########.##.....##.##....##..######..########.####..#######..##....##
.###...###...##.##....##..###...##....##.......##.....##.###...##.##....##....##.....##..##.....##.###...##
.####.####..##...##...##..####..##....##.......##.....##.####..##.##..........##.....##..##.....##.####..##
.##.###.##.##.....##..##..##.##.##....######...##.....##.##.##.##.##..........##.....##..##.....##.##.##.##
.##.....##.#########..##..##..####....##.......##.....##.##..####.##..........##.....##..##.....##.##..####
.##.....##.##.....##..##..##...###....##.......##.....##.##...###.##....##....##.....##..##.....##.##...###
.##.....##.##.....##.####.##....##....##........#######..##....##..######.....##....####..#######..##....##
*/

func main() {
	launchTimeUTC := time.Date(2025, 5, 24, 16, 5, 0, 0, time.UTC)
	numberOfDays := int(time.Since(launchTimeUTC).Hours() / 24)
	BackfillStatsFrom(&launchTimeUTC, numberOfDays)
}

func BackfillStatsFrom(startTs *time.Time, days int) {
	ctx := context.Background()
	client, err := ethclient.Dial(infuraApiUrl + infuraSecret)
	if err != nil {
		fmt.Println("error while dialing client:", err)
		return
	}
	defer client.Close()

	latestBlock, err := getChainLastBlockNumber(client)
	if err != nil {
		fmt.Println("error getting last block number:", err)
		return
	}

	time.Sleep(sleepTime)
	cspAddresses, err := getAllCSPAddress(client)
	if err != nil {
		fmt.Println("Error while retrieving csp addresses:", err)
		return
	}

	var statstiche []model.Stats
	oldStats := &model.Stats{
		TotalTokenBurn:           big.NewInt(0),
		TotalNdContractTokenBurn: big.NewInt(0),
		TotalMinted:              big.NewInt(0),
		TotalPOAIRewards:         big.NewInt(0),
		LastBlockNumber:          0,
	}

	for i := 0; i < days; i++ {
		fmt.Println("Processing day", i+1, "of", days)
		dayEnd := startTs.Add(24 * time.Hour)
		fromBlock := oldStats.LastBlockNumber + 1
		time.Sleep(sleepTime)
		toBlock, err := findBlockByTimestampSmart(ctx, client, dayEnd, fromBlock, latestBlock)
		if err != nil {
			fmt.Println("error locating toBlock:", err)
			return
		}

		fmt.Println("  fromBlock:", fromBlock, " toBlock:", toBlock, " (day end:", dayEnd, ")")
		time.Sleep(sleepTime)
		allocEvents, err := fetchAllocationEvents(cspAddresses, fromBlock, toBlock, client)
		if err != nil {
			fmt.Println("Error fetching allocation events:", err)
			return
		}

		dailyPoaiReward := big.NewInt(0)
		for _, e := range allocEvents {
			dailyPoaiReward.Add(dailyPoaiReward, &e.UsdcAmountPayed)
		}
		time.Sleep(sleepTime)
		dailyMinted, err := getPeriodMintedAmount(fromBlock, toBlock, client)
		if err != nil {
			fmt.Println("error getting daily minted:", err)
			return
		}

		time.Sleep(sleepTime)
		dailyTokenBurn, err := getPeriodBurnedAmount(fromBlock, toBlock, client)
		if err != nil {
			fmt.Println("error getting daily token burn:", err)
			return
		}

		time.Sleep(sleepTime)
		dailyNdContractTokenBurn, err := getPeriodNdContractBurnedAmount(fromBlock, toBlock, client)
		if err != nil {
			fmt.Println("error getting daily nd contract token burn:", err)
			return
		}

		time.Sleep(sleepTime)
		totalSupply, err := getTotalSupplyAt(ctx, client, toBlock)
		if err != nil {
			fmt.Println("error getting historical totalSupply:", err)
			return
		}

		time.Sleep(sleepTime)
		teamWalletsSupply, err := getTeamWalletsSupplyAt(ctx, client, toBlock)
		if err != nil {
			fmt.Println("error getting historical team wallets supply:", err)
			return
		}

		time.Sleep(sleepTime)
		dailyUsdcLocked, err := getUsdcLockedAt(ctx, client, cspAddresses, toBlock)
		if err != nil {
			fmt.Println("error getting historical USDC locked:", err)
			return
		}

		time.Sleep(sleepTime)
		dailyActiveJobs, err := getActiveJobsAt(ctx, client, toBlock)
		if err != nil {
			fmt.Println("error getting historical active jobs:", err)
			return
		}

		stats := model.Stats{
			CreationTimestamp:        dayEnd.UTC(),
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
			LastBlockNumber:          toBlock,
		}
		oldStats = &stats
		statstiche = append(statstiche, stats)
		startTs = &dayEnd
		time.Sleep(sleepTime)
	}

	//save on json file
	data, _ := json.MarshalIndent(statstiche, "", "  ")
	os.WriteFile("scripts/stats.json", data, 0644)

}

/*
..######...########.########....########.##.....##.##....##..######..########.####..#######..##....##..######.
.##....##..##..........##.......##.......##.....##.###...##.##....##....##.....##..##.....##.###...##.##....##
.##........##..........##.......##.......##.....##.####..##.##..........##.....##..##.....##.####..##.##......
.##...####.######......##.......######...##.....##.##.##.##.##..........##.....##..##.....##.##.##.##..######.
.##....##..##..........##.......##.......##.....##.##..####.##..........##.....##..##.....##.##..####.......##
.##....##..##..........##.......##.......##.....##.##...###.##....##....##.....##..##.....##.##...###.##....##
..######...########....##.......##........#######..##....##..######.....##....####..#######..##....##..######.
*/

type headerCache struct {
	mu sync.RWMutex
	m  map[int64]uint64 // blockNumber -> unix ts
}

func newHeaderCache() *headerCache { return &headerCache{m: make(map[int64]uint64)} }

func (c *headerCache) get(n int64) (uint64, bool) {
	c.mu.RLock()
	ts, ok := c.m[n]
	c.mu.RUnlock()
	return ts, ok
}

func (c *headerCache) put(n int64, ts uint64) {
	c.mu.Lock()
	c.m[n] = ts
	c.mu.Unlock()
}

func findBlockByTimestampSmart(ctx context.Context, client *ethclient.Client, t time.Time, low, high int64) (int64, error) {
	if low < 0 {
		low = 0
	}
	target := t.UTC().Unix()

	if high == 0 || high < low {
		h, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			return 0, err
		}
		high = h.Number.Int64()
	}

	cache := newHeaderCache()
	getTs := func(n int64) (int64, error) {
		if n < 0 {
			return 0, errors.New("negative block number")
		}
		if ts, ok := cache.get(n); ok {
			return int64(ts), nil
		}
		h, err := client.HeaderByNumber(ctx, big.NewInt(n))
		if err != nil {
			return 0, err
		}
		cache.put(n, h.Time)
		return int64(h.Time), nil
	}

	lowTs, err := getTs(low)
	if err != nil {
		return 0, err
	}
	highTs, err := getTs(high)
	if err != nil {
		return 0, err
	}
	if target <= lowTs {
		return low, nil
	}
	if target > highTs {
		return high + 1, nil
	}

	const (
		maxIters          = 32
		switchToBinaryGap = int64(10_000)
	)

	L, H := low, high
	Lts, Hts := lowTs, highTs

	for iter := 0; iter < maxIters && L < H; iter++ {
		time.Sleep(100 * time.Millisecond)
		spanBlocks := H - L
		spanTime := Hts - Lts
		if spanBlocks <= 0 || spanTime <= 0 {
			break
		}

		frac := float64(target-Lts) / float64(spanTime)
		if frac < 0 {
			frac = 0
		} else if frac > 1 {
			frac = 1
		}
		guess := L + int64(math.Round(frac*float64(spanBlocks)))

		if guess <= L {
			guess = L + 1
		} else if guess >= H {
			guess = H - 1
		}

		guessTs, err := getTs(guess)
		if err != nil {
			return 0, err
		}

		if guessTs >= target {
			H, Hts = guess, guessTs
		} else {
			L, Lts = guess+1, 0
			if L <= H {
				Lts, err = getTs(L)
				if err != nil {
					return 0, err
				}
			}
		}

		if H-L <= switchToBinaryGap {
			break
		}
	}

	// ----- Binary tighten -----
	for L < H {
		mid := L + (H-L)/2
		mts, err := getTs(mid)
		if err != nil {
			return 0, err
		}
		if mts >= target {
			H = mid
		} else {
			L = mid + 1
		}
	}
	return L, nil
}

func getTotalSupplyAt(ctx context.Context, client *ethclient.Client, atBlock int64) (*big.Int, error) {
	tokenAddress := common.HexToAddress(r1ContractAddress)
	const erc20ABI = `[{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return big.NewInt(0), err
	}

	data, err := parsedABI.Pack("totalSupply")
	if err != nil {
		return big.NewInt(0), err
	}

	msg := ethereum.CallMsg{To: &tokenAddress, Data: data}
	result, err := client.CallContract(ctx, msg, big.NewInt(atBlock))
	if err != nil {
		return big.NewInt(0), err
	}

	var totalSupply *big.Int
	if err := parsedABI.UnpackIntoInterface(&totalSupply, "totalSupply", result); err != nil {
		return big.NewInt(0), nil
	}
	return totalSupply, nil
}

func getTeamWalletsSupplyAt(ctx context.Context, client *ethclient.Client, atBlock int64) (*big.Int, error) {
	tokenAddress := common.HexToAddress(r1ContractAddress)
	const erc20ABI = `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return big.NewInt(0), err
	}

	total := big.NewInt(0)
	for _, addrStr := range teamAddresses {
		addr := common.HexToAddress(addrStr)
		data, err := parsedABI.Pack("balanceOf", addr)
		if err != nil {
			return big.NewInt(0), err
		}
		msg := ethereum.CallMsg{To: &tokenAddress, Data: data}
		res, err := client.CallContract(ctx, msg, big.NewInt(atBlock))
		if err != nil {
			return big.NewInt(0), fmt.Errorf("balanceOf historical call failed for %s: %w", addrStr, err)
		}
		var bal *big.Int
		if err := parsedABI.UnpackIntoInterface(&bal, "balanceOf", res); err != nil {
			return big.NewInt(0), nil
		}
		total.Add(total, bal)
	}
	return total, nil
}

func getUsdcLockedAt(ctx context.Context, client *ethclient.Client, cspAddresses map[string]string, atBlock int64) (*big.Int, error) {
	tokenAddress := common.HexToAddress(usdcContractAddress)
	const erc20ABI = `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return big.NewInt(0), err
	}

	total := big.NewInt(0)
	for addrStr := range cspAddresses {
		addr := common.HexToAddress(addrStr)
		data, err := parsedABI.Pack("balanceOf", addr)
		if err != nil {
			return big.NewInt(0), err
		}
		msg := ethereum.CallMsg{To: &tokenAddress, Data: data}
		res, err := client.CallContract(ctx, msg, big.NewInt(atBlock))
		if err != nil {
			return big.NewInt(0), fmt.Errorf("USDC balanceOf historical call failed for %s: %w", addrStr, err)
		}
		var bal *big.Int
		if err := parsedABI.UnpackIntoInterface(&bal, "balanceOf", res); err != nil {
			return big.NewInt(0), nil
		}
		total.Add(total, bal)
	}
	return total, nil
}

func getActiveJobsAt(ctx context.Context, client *ethclient.Client, atBlock int64) (int, error) {
	tokenAddress := common.HexToAddress(poaiManagerAddress)
	const poaiManagerAbi = `[{"inputs":[],"name":"nextJobId","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(poaiManagerAbi))
	if err != nil {
		return 0, err
	}

	data, err := parsedABI.Pack("nextJobId")
	if err != nil {
		return 0, err
	}

	msg := ethereum.CallMsg{To: &tokenAddress, Data: data}
	res, err := client.CallContract(ctx, msg, big.NewInt(atBlock))
	if err != nil {
		return 0, err
	}

	var nextJobId *big.Int
	if err := parsedABI.UnpackIntoInterface(&nextJobId, "nextJobId", res); err != nil {
		return 0, nil
	}
	return int(new(big.Int).Sub(nextJobId, big.NewInt(1)).Int64()), nil
}

func getChainLastBlockNumber(client *ethclient.Client) (int64, error) {

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, errors.New("error while retrieving header from client: " + err.Error())
	}

	return header.Number.Int64(), nil
}

func getAllCSPAddress(client *ethclient.Client) (map[string]string, error) { // map[cspAddress]ownerAddress
	contractAddress := common.HexToAddress(poaiManagerAddress)
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

func fetchAllocationEvents(cspOwners map[string]string, from, to int64, client *ethclient.Client) ([]model.Allocation, error) {
	var addresses []common.Address
	for k := range cspOwners {
		addresses = append(addresses, common.HexToAddress(k))
	}

	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	eventSignatureAsBytes := []byte(allocationEventSignature)
	eventHash := crypto.Keccak256Hash(eventSignatureAsBytes)

	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: addresses,
		Topics:    [][]common.Hash{{eventHash}},
	}

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
	parsedABI, err := abi.JSON(strings.NewReader(allocLogsAbi))
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

func getPeriodMintedAmount(from, to int64, client *ethclient.Client) (*big.Int, error) {
	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	tokenAddress := common.HexToAddress(r1ContractAddress)

	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	mintedTotal := big.NewInt(0)
	alreadySeen := make(map[string]bool)

	for {
		mintedQuery := ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{tokenAddress},
			Topics: [][]common.Hash{
				{transferEventSigHash},
				{zeroTopic}, // This is the "to" address, which we don't filter on
			},
		}

		mintedLogs, err := client.FilterLogs(context.Background(), mintedQuery)
		if err != nil {
			return big.NewInt(0), errors.New("error while filtering minted logs")
		}

		for _, vLog := range mintedLogs {
			if _, exist := alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))]; exist {
				continue
			}

			if len(vLog.Data) != 32 {
				return big.NewInt(0), errors.New("unexpected data length in minted log")
			}

			amount := new(big.Int).SetBytes(vLog.Data)
			mintedTotal.Add(mintedTotal, amount)
			alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))] = true

			if vLog.BlockNumber > fromBlock.Uint64() {
				fromBlock = big.NewInt(int64(vLog.BlockNumber))
			}
		}

		if len(mintedLogs) < 10000 {
			break
		}
	}

	return mintedTotal, nil
}

func getPeriodBurnedAmount(from, to int64, client *ethclient.Client) (*big.Int, error) {
	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	tokenAddress := common.HexToAddress(r1ContractAddress)
	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	burnedTotal := big.NewInt(0)
	alreadySeen := make(map[string]bool)

	for {
		burnedQuery := ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{tokenAddress},
			Topics: [][]common.Hash{
				{transferEventSigHash},
				{},
				{zeroTopic},
			},
		}

		burnedLogs, err := client.FilterLogs(context.Background(), burnedQuery)
		if err != nil {
			return big.NewInt(0), errors.New("error while filtering burned logs: " + err.Error())
		}

		for _, vLog := range burnedLogs {
			if _, exist := alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))]; exist {
				continue
			}

			if len(vLog.Data) != 32 {
				return big.NewInt(0), errors.New("unexpected data length in burned log")
			}

			amount := new(big.Int).SetBytes(vLog.Data)
			burnedTotal.Add(burnedTotal, amount)
			alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))] = true

			if vLog.BlockNumber > fromBlock.Uint64() {
				fromBlock = big.NewInt(int64(vLog.BlockNumber))
			}
		}

		if len(burnedLogs) < 10000 {
			break
		}
	}

	return burnedTotal, nil
}

func getPeriodNdContractBurnedAmount(from, to int64, client *ethclient.Client) (*big.Int, error) {
	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	tokenAddress := common.HexToAddress(r1ContractAddress)

	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	ndContractAddress := common.HexToAddress(ndContractAddress)
	ndContractTopic := common.BytesToHash(ndContractAddress.Bytes())

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	zeroTopic := common.BytesToHash(zeroAddress.Bytes())

	burnedTotal := big.NewInt(0)
	alreadySeen := make(map[string]bool)

	for {
		burnedQuery := ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{tokenAddress},
			Topics: [][]common.Hash{
				{transferEventSigHash},
				{ndContractTopic},
				{zeroTopic},
			},
		}

		burnedLogs, err := client.FilterLogs(context.Background(), burnedQuery)
		if err != nil {
			return big.NewInt(0), errors.New("error while filtering burned logs: " + err.Error())
		}

		for _, vLog := range burnedLogs {
			if _, exist := alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))]; exist {
				continue
			}

			if len(vLog.Data) != 32 {
				return big.NewInt(0), errors.New("unexpected data length in burned log")
			}

			amount := new(big.Int).SetBytes(vLog.Data)
			burnedTotal.Add(burnedTotal, amount)
			alreadySeen[vLog.TxHash.String()+strconv.Itoa(int(vLog.TxIndex))+strconv.Itoa(int(vLog.Index))] = true

			if vLog.BlockNumber > fromBlock.Uint64() {
				fromBlock = big.NewInt(int64(vLog.BlockNumber))
			}
		}

		if len(burnedLogs) < 10000 {
			break
		}
	}

	return burnedTotal, nil
}
