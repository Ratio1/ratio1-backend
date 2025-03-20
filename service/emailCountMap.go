package service

import (
	"errors"
	"sync"
)

const maxEmailsPerAddress = 10

var (
	mut  sync.Mutex
	cMap map[string]int
)

func increaseEmailCount(address string) error {
	mut.Lock()
	defer mut.Unlock()

	if cMap == nil {
		cMap = make(map[string]int)
	}

	newCount := 1
	if count, found := cMap[address]; found {
		newCount = count + 1
	}

	if newCount == maxEmailsPerAddress {
		return errors.New("too many email requests for address")
	}

	cMap[address] = newCount

	return nil
}
