package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
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

func fetchOldAllocationEvents(cspOwners map[string]string, from, to int64, client *ethclient.Client) ([]model.Allocation, error) {
	var addresses []common.Address
	for k := range cspOwners {
		addresses = append(addresses, common.HexToAddress(k))
	}

	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	eventSignatureAsBytes := []byte(OldAllocationEventSignature)
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
		event, err := decodeOldAllocLogs(vLog)
		if err != nil {
			fmt.Println("error while decoding logs: " + err.Error())
			continue
		}

		for i, e := range event {
			event[i].CspOwner = cspOwners[e.CspAddress]
		}

		events = append(events, event...)
	}
	return events, nil
}

func decodeOldAllocLogs(vLog types.Log) ([]model.Allocation, error) {
	parsedABI, err := abi.JSON(strings.NewReader(OldAllocLogsAbi))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	event := struct {
		ActiveNodes []common.Address
		TotalAmount *big.Int
	}{}

	err = parsedABI.UnpackIntoInterface(&event, "RewardsAllocated", vLog.Data)
	if err != nil {
		return nil, errors.New("error while unpacking interface: " + err.Error())
	}

	amount := big.NewInt(0).Quo(event.TotalAmount, big.NewInt(int64(len(event.ActiveNodes))))
	var result []model.Allocation
	jobIDBig := new(big.Int).SetBytes(vLog.Topics[1].Bytes())
	var jobID uint64
	if jobIDBig.BitLen() > 64 {
		return nil, fmt.Errorf("jobId too large for uint64: %s", jobIDBig.String())
	}
	jobID = jobIDBig.Uint64()
	for _, nodeAddr := range event.ActiveNodes {
		alloc := model.Allocation{
			CspAddress:  vLog.Address.String(),
			TxHash:      vLog.TxHash.Hex(),
			BlockNumber: int64(vLog.BlockNumber),

			NodeAddress:     nodeAddr.String(),
			JobId:           strconv.Itoa(int(jobID)),
			UsdcAmountPayed: amount.String(),
		}
		result = append(result, alloc)
	}
	return result, nil
}

func getTotalSupplyAt(ctx context.Context, client *ethclient.Client, atBlock int64) (*big.Int, error) {
	tokenAddress := common.HexToAddress(R1ContractAddress)
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
	tokenAddress := common.HexToAddress(R1ContractAddress)
	const erc20ABI = `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return big.NewInt(0), err
	}

	total := big.NewInt(0)
	for _, addrStr := range TeamAddresses {
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
	tokenAddress := common.HexToAddress(UsdcContractAddress)
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
	tokenAddress := common.HexToAddress(PoaiManagerAddress)
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
	contractAddress := common.HexToAddress(PoaiManagerAddress)
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

	eventSignatureAsBytes := []byte(AllocationEventSignature)
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
	parsedABI, err := abi.JSON(strings.NewReader(AllocLogsAbi))
	if err != nil {
		return nil, errors.New("error while parsing abi: " + err.Error())
	}

	event := struct {
		NodeAddress common.Address
		NodeOwner   common.Address
		UsdcAmount  *big.Int
	}{}

	err = parsedABI.UnpackIntoInterface(&event, "RewardsAllocatedV2", vLog.Data)
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

		NodeAddress:     event.NodeAddress.String(),
		UserAddress:     event.NodeOwner.String(),
		JobId:           strconv.Itoa(int(jobID)),
		UsdcAmountPayed: event.UsdcAmount.String(),
	}
	return &result, nil
}

func getPeriodMintedAmount(from, to int64, client *ethclient.Client) (*big.Int, error) {
	fromBlock := big.NewInt(from)
	toBlock := big.NewInt(to)

	tokenAddress := common.HexToAddress(R1ContractAddress)

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

	tokenAddress := common.HexToAddress(R1ContractAddress)
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

	tokenAddress := common.HexToAddress(R1ContractAddress)

	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventSigHash := crypto.Keccak256Hash(transferEventSignature)

	ndContractAddress := common.HexToAddress(NdContractAddress)
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
