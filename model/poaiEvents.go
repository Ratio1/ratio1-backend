package model

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Allocation struct {
	Id          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	BlockNumber int64  `gorm:"type:bigint;not null" json:"blockNumber"`
	TxHash      string `gorm:"type:varchar(66);not null" json:"txHash"`

	JobId       string `gorm:"type:text;not null" json:"jobId"`
	JobName     string `gorm:"type:text;default:null" json:"jobName"`
	JobType     string `gorm:"type:text;default:null" json:"jobType"`
	ProjectName string `gorm:"type:text;default:null" json:"projectName"`

	NodeAddress     string     `gorm:"type:varchar(66);not null" json:"nodeAddress"`
	UserAddress     string     `gorm:"type:varchar(66);not null;index" json:"userAddress"`
	CspAddress      string     `gorm:"type:varchar(66);not null;index" json:"cspAddress"`
	CspOwner        string     `gorm:"type:varchar(66);not null" json:"cspOwner"`
	UsdcAmountPayed string     `gorm:"type:numeric;default:null" json:"usdcAmountPayed"`
	DraftId         *uuid.UUID `gorm:"type:uuid;default:null" json:"invoiceId"`

	CspProfile  UserInfo `gorm:"foreignKey:CspOwner;references:BlockchainAddress" json:"cspProfile"`
	UserProfile UserInfo `gorm:"foreignKey:UserAddress;references:BlockchainAddress" json:"userProfile"`
}

func (a *Allocation) GetUsdcAmountPayed() *big.Int {
	amount, ok := new(big.Int).SetString(a.UsdcAmountPayed, 10)
	if !ok {
		return big.NewInt(0)
	}
	return amount
}

func (a *Allocation) SetUsdcAmountPayed(amount *big.Int) {
	if amount == nil {
		a.UsdcAmountPayed = "0"
	} else {
		a.UsdcAmountPayed = amount.String()
	}
}

type InvoiceDraft struct {
	DraftId           uuid.UUID `gorm:"type:uuid;primaryKey" json:"invoiceId"`
	CreationTimestamp time.Time `gorm:"type:timestamp;not null" json:"creationTimestamp"`
	UserAddress       string    `gorm:"type:varchar(66);not null;index" json:"userAddress"`
	CspOwner          string    `gorm:"type:varchar(66);not null;index" json:"cspOwner"`
	TotalUsdcAmount   float64   `gorm:"type:numeric" json:"totalUsdcAmount"`
	VatApplied        float64   `gorm:"type:numeric" json:"vatApplied"`
	InvoiceSeries     string    `gorm:"type:text;default:null" json:"invoiceSeries"`
	InvoiceNumber     int       `gorm:"type:integer;default:null" json:"invoiceNumber"`
	ExtraText         *string   `gorm:"type:text;default:null" json:"extraText"`
	ExtraTaxes        *string   `gorm:"type:jsonb;default:'{}'" json:"extraTaxes"`

	CspProfile  UserInfo `gorm:"foreignKey:CspOwner;references:BlockchainAddress" json:"cspProfile"`
	UserProfile UserInfo `gorm:"foreignKey:UserAddress;references:BlockchainAddress" json:"userProfile"`
}

type Preference struct {
	UserAddress   string  `gorm:"type:varchar(66);primaryKey" json:"userAddress"`
	InvoiceSeries string  `gorm:"type:text;default:null"      json:"invoiceSeries"`
	NextNumber    int     `gorm:"type:integer;default:1"      json:"nextNumber"`
	Country       float64 `gorm:"type:numeric"                json:"country"`
	Ue            float64 `gorm:"type:numeric"                json:"ue"`
	ExtraUe       float64 `gorm:"type:numeric"                json:"extraUe"`
	ExtraText     *string `gorm:"type:text;default:null"      json:"extraText"`
	LocalCurrency string  `gorm:"type:varchar(3)"             json:"localCurrency"` // es. "EUR", "USD"
	ExtraTaxes    *string `gorm:"type:jsonb;default:'{}'"     json:"extraTaxes"`
}

type ExtraTax struct {
	Description string
	TaxType     TaxTypeEnum
	Value       float64
}

func (p *Preference) GetExtraTaxes() ([]ExtraTax, error) {
	if isEmptyJSONList(p.ExtraTaxes) {
		return []ExtraTax{}, nil
	}
	if !json.Valid([]byte(*p.ExtraTaxes)) {
		return nil, errors.New("payload is not a valid JSON")
	}
	var taxes []ExtraTax
	if err := json.Unmarshal([]byte(*p.ExtraTaxes), &taxes); err != nil {
		return nil, err
	}
	return taxes, nil
}

func (p *InvoiceDraft) GetExtraTaxes() ([]ExtraTax, error) {
	if isEmptyJSONList(p.ExtraTaxes) {
		return []ExtraTax{}, nil
	}
	if !json.Valid([]byte(*p.ExtraTaxes)) {
		return nil, errors.New("payload is not a valid JSON")
	}
	var taxes []ExtraTax
	if err := json.Unmarshal([]byte(*p.ExtraTaxes), &taxes); err != nil {
		return nil, err
	}
	return taxes, nil
}

func (p *Preference) SetExtraTaxes(taxes []ExtraTax) error {
	data, err := json.Marshal(taxes)
	if err != nil {
		return err
	}
	str := string(data) // sarà sempre "[]" per empty, non "{}"
	p.ExtraTaxes = &str
	return nil
}

type TaxTypeEnum int

const (
	Fixed TaxTypeEnum = iota
	Percentage
)

func isEmptyJSONList(s *string) bool {
	if s == nil {
		return true
	}
	trim := strings.TrimSpace(*s)
	return trim == "" || trim == "[]" || trim == "null" || trim == "{}"
}
