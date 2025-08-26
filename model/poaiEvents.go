package model

import (
	"math/big"

	"github.com/google/uuid"
)

type Allocation struct {
	BlockNumber int64
	TxHash      string
	JobId       string
	NodeAddress string
	UserAddress string
	//CspProfile      UserInfo
	//UserProfile     UserInfo
	CspAddress      string
	CspOwner        string
	UsdcAmountPayed big.Int
	InvoiceId       *uuid.UUID
}
