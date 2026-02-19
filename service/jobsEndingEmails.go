package service

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/ratio1abi"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type EndingJob struct {
	JobID                    *big.Int
	ProjectHash              [32]byte
	RequestTimestamp         *big.Int
	StartTimestamp           *big.Int
	LastNodesChangeTimestamp *big.Int
	JobType                  *big.Int
	PricePerEpoch            *big.Int
	LastExecutionEpoch       *big.Int
	NumberOfNodesRequested   *big.Int
	Balance                  *big.Int
	LastAllocatedEpoch       *big.Int
	ActiveNodes              []common.Address
	EscrowAddress            common.Address
	EscrowOwner              common.Address
	NotifyBeforeEpochs       int64
	JobName                  string
}

type endingJobOnChain struct {
	ID                       *big.Int         `abi:"id"`
	ProjectHash              [32]byte         `abi:"projectHash"`
	RequestTimestamp         *big.Int         `abi:"requestTimestamp"`
	StartTimestamp           *big.Int         `abi:"startTimestamp"`
	LastNodesChangeTimestamp *big.Int         `abi:"lastNodesChangeTimestamp"`
	JobType                  *big.Int         `abi:"jobType"`
	PricePerEpoch            *big.Int         `abi:"pricePerEpoch"`
	LastExecutionEpoch       *big.Int         `abi:"lastExecutionEpoch"`
	NumberOfNodesRequested   *big.Int         `abi:"numberOfNodesRequested"`
	Balance                  *big.Int         `abi:"balance"`
	LastAllocatedEpoch       *big.Int         `abi:"lastAllocatedEpoch"`
	ActiveNodes              []common.Address `abi:"activeNodes"`
	EscrowAddress            common.Address   `abi:"escrowAddress"`
	EscrowOwner              common.Address   `abi:"escrowOwner"`
}

func manageEndingJobsAndSendEmails(jobNamesForId map[string]*JobDetailsResult) error {
	jobs, err := getEndingJobsWithPeriod()
	if err != nil {
		return err
	}

	// the email should be at maximum 1 for user + all ending jobs details inside the html template ( if a user has 3 ending jobs, it will recieve 1 email with the 3 details inside)
	// so compact the ending jobs per owner address
	usersWithJobs := make(map[string][]EndingJob)
	for _, job := range jobs {
		ownerAddress := strings.ToLower(job.EscrowOwner.Hex())
		usersWithJobs[ownerAddress] = append(usersWithJobs[ownerAddress], job)
	}

	for ownerAddress := range usersWithJobs {
		sort.Slice(usersWithJobs[ownerAddress], func(i, j int) bool {
			left := usersWithJobs[ownerAddress][i]
			right := usersWithJobs[ownerAddress][j]
			if left.NotifyBeforeEpochs != right.NotifyBeforeEpochs {
				return left.NotifyBeforeEpochs < right.NotifyBeforeEpochs
			}
			return compareBigInt(left.JobID, right.JobID) < 0
		})
	}

	for ownerAddress := range usersWithJobs {
		for i := range usersWithJobs[ownerAddress] {
			details := jobNamesForId[usersWithJobs[ownerAddress][i].JobID.String()]
			usersWithJobs[ownerAddress][i].JobName = details.JobName
		}
	}

	sendEmailForEndingJobs(usersWithJobs)
	return nil
}

func getEndingJobsWithPeriod() ([]EndingJob, error) {
	// 1 day before + 3 days before + 5 days before
	periods := []int64{1, 3, 5}
	contractAddress := common.HexToAddress(config.Config.ReaderAddress)

	parsedABI, err := abi.JSON(strings.NewReader(ratio1abi.GetJobsByLastExecutionEpochDeltaAbi))
	if err != nil {
		return nil, errors.New("error while parsing reader abi: " + err.Error())
	}

	client, err := ethclient.Dial(config.Config.Infura.ApiUrl + config.Config.Infura.Secret)
	if err != nil {
		return nil, errors.New("error while dialing client")
	}
	defer client.Close()

	endingJobs := make([]EndingJob, 0)
	for _, period := range periods {
		data, err := parsedABI.Pack("getJobsByLastExecutionEpochDelta", big.NewInt(period))
		if err != nil {
			return nil, errors.New("error while packing getJobsByLastExecutionEpochDelta call: " + err.Error())
		}

		msg := ethereum.CallMsg{
			To:   &contractAddress,
			Data: data,
		}

		result, err := client.CallContract(context.Background(), msg, nil)
		if err != nil {
			return nil, errors.New("error while calling getJobsByLastExecutionEpochDelta: " + err.Error())
		}

		var jobsOnChain []endingJobOnChain
		err = parsedABI.UnpackIntoInterface(&jobsOnChain, "getJobsByLastExecutionEpochDelta", result)
		if err != nil {
			return nil, errors.New("error while unpacking getJobsByLastExecutionEpochDelta response: " + err.Error())
		}

		for _, job := range jobsOnChain {
			jobID := job.ID
			if jobID == nil {
				jobID = big.NewInt(0)
			}
			endingJobs = append(endingJobs, EndingJob{
				JobID:                    jobID,
				ProjectHash:              job.ProjectHash,
				RequestTimestamp:         job.RequestTimestamp,
				StartTimestamp:           job.StartTimestamp,
				LastNodesChangeTimestamp: job.LastNodesChangeTimestamp,
				JobType:                  job.JobType,
				PricePerEpoch:            job.PricePerEpoch,
				LastExecutionEpoch:       job.LastExecutionEpoch,
				NumberOfNodesRequested:   job.NumberOfNodesRequested,
				Balance:                  job.Balance,
				LastAllocatedEpoch:       job.LastAllocatedEpoch,
				ActiveNodes:              job.ActiveNodes,
				EscrowAddress:            job.EscrowAddress,
				EscrowOwner:              job.EscrowOwner,
				NotifyBeforeEpochs:       period,
			})
		}
	}

	return endingJobs, nil
}

func sendEmailForEndingJobs(usersWithJobs map[string][]EndingJob) {
	// TODO if data is missing we skip and continue (no hard failure). to be done differently
	for ownerAddress, jobs := range usersWithJobs {
		if len(jobs) == 0 {
			continue
		}

		account, found, err := storage.GetAccountByAddress(ownerAddress)
		if err != nil {
			log.Error("error while retrieving account for address %s: %v", ownerAddress, err)
			continue
		}
		if !found || account == nil || account.Email == nil {
			continue
		}

		email := strings.TrimSpace(*account.Email)
		if email == "" {
			continue
		}

		err = SendJobsEndingEmail(email, jobs)
		if err != nil {
			log.Error("error while sending ending jobs email to %s: %v", email, err)
			continue
		}
	}
}

func compareBigInt(left, right *big.Int) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return -1
	case right == nil:
		return 1
	default:
		return left.Cmp(right)
	}
}
