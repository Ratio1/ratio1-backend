package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/ethereum/go-ethereum/ethclient"
)

/*
MAKE sure to set all the needed variabels berfore running
*/

/*
	func main() {
		fmt.Println("beginning of all allocatoin script")
		allocations := getAllAllocations()
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

func getAllAllocations() []model.Allocation {
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
	/*
		for _, a := range allocEvents {
			_ = a.GetJobDetails(config.Config.JobDetailsApi) //ignore the error
		}*/

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

	/*
		for _, a := range allocEvents {
			_ = a.GetJobDetails(config.Config.JobDetailsApi) //ignore the error
		}*/

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

	olddata, _ := json.Marshal(oldAllocEvent)
	_ = os.WriteFile("old.json", olddata, 0644)

	return newAllocEvent
}
