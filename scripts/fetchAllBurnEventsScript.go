package main

import (
	"fmt"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/ethereum/go-ethereum/ethclient"
)

/*func main() {
	DBConnect()
	events := GetBurnEvents()
	if len(events) == 0 {
		fmt.Println("no events to insert")
		return
	}
	fileByte, err := json.Marshal(events)
	if err != nil {
		fmt.Println("error while marshalling events:", err)
		return
	}
	os.WriteFile("burnEvents.json", fileByte, 0644)

	for _, e := range events {
		err := createBurnEvent(&e)
		if err != nil {
			fmt.Println("error while inserting burn event:", err, "for event:", e)
		}
	}
}*/

func GetBurnEvents() []model.BurnEvent {
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

	cspAddresses, err := getAllCSPAddress(client) // map[cspSCAddress]ownerAddress
	if err != nil {
		fmt.Println("Error while retrieving csp addresses:", err)
		return nil
	}

	// Get all new allocation to create them on DB and fetch all nodeAddre-nodeOwner for old event (missing match)
	burnEvents, err := fetchBurnEvents(cspAddresses, 0, latestBlock, client)
	if err != nil {
		fmt.Println("Error fetching allocation events:", err)
		return nil
	}

	fmt.Println("lenght of burnEvents: ", len(burnEvents))

	blocks := make(map[int64]*time.Time)
	for _, a := range burnEvents {
		blocks[a.BlockNumber] = nil
	}

	for k := range blocks {
		v, err := getBlockTimestamp(k, client)
		if err != nil {
			fmt.Println("cannot fetch correct timestamp for block: ", k, "with error: ", err.Error())
			continue
		}
		blocks[k] = &v
		time.Sleep(SleepTime)
	}

	currencyMap, err := getFreeCurrencyValues() //map[USD,EUR...]ratio always based 1 usd -> value
	if err != nil {
		fmt.Println("could not fetch currency map: ", err.Error())
		return nil
	}

	/* get preferences for eache csp owner*/
	cspPreferences := make(map[string]*model.Preference) // map[cspOwnerAddress]Preference
	for _, v := range cspAddresses {
		preference, err := getPreferenceByAddress(v)
		if err != nil || preference == nil {
			fmt.Println("no preference found for csp owner: ", v)
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

	return burnEvents
}
