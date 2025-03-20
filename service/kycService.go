package service

import (
	"errors"
)

var (
	ErrorKycNotFound     = errors.New("kyc not found")
	ErrorKycNotCompleted = errors.New("kyc not completed")
)

type KycInfo struct {
	KycRef         *string
	KycStatus      string
	ReceiveUpdates bool
}
