package model

import (
	"time"
)

type Account struct {
	Address               string    `gorm:"primarykey;length:42;" json:"address"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
	Email                 *string   `gorm:"unique;default:null" json:"email"`
	EmailConfirmed        bool      `json:"emailConfirmed"`
	PendingEmail          string    `gorm:"default:null" json:"pendingEmail"`
	PendingReceiveUpdates bool      `gorm:"not null;default:false" json:"pendingReceiveUpdates"`
	IsBlacklisted         bool      `gorm:"not null;default:false" json:"isBlacklisted"`
	BlacklistedReason     *string   `gorm:"default:null" json:"blacklistedReason"`
}

type AccountDto struct {
	Email             string  `json:"email"`
	EmailConfirmed    bool    `json:"emailConfirmed"`
	PendingEmail      string  `json:"pendingEmail"`
	ApplicantType     string  `json:"applicantType"`
	Address           string  `json:"address"`
	Uuid              string  `json:"uuid"`
	KycStatus         string  `json:"kycStatus"`
	ReceiveUpdates    bool    `json:"receiveUpdates"`
	IsActive          bool    `json:"isActive"`
	IsBlacklisted     bool    `json:"isBlacklisted"`
	BlacklistedReason *string `json:"blacklistedReason"`
	UsdBuyLimit       int     `json:"usdBuyLimit"`
	VatPercentage     int64   `json:"vatPercentage"`
}
