package model

type Seller struct {
	SellerCode string  `gorm:"primaryKey;" json:"sellerCode"`
	AccountID  string  `gorm:"not null" json:"accountId"`
	IsDisabled bool    `gorm:"default:false" json:"isDisabled"`
	Account    Account `gorm:"foreignKey:AccountID;references:Address" json:"account"`
}
