package model

type UserInfo struct {
	BlockchainAddress  string  `gorm:"primaryKey;type:varchar(66)" json:"blockchainAddress"`
	Email              string  `gorm:"type:text" json:"email"`
	Name               *string `gorm:"type:text;default:null" json:"name"`
	Surname            *string `gorm:"type:text;default:null" json:"surname"`
	CompanyName        *string `gorm:"type:text;default:null" json:"companyName"`
	IdentificationCode string  `gorm:"type:text" json:"identificationCode"`
	Address            string  `gorm:"type:text" json:"address"`
	State              string  `gorm:"type:text" json:"state"`
	City               string  `gorm:"type:text" json:"city"`
	Country            string  `gorm:"type:text" json:"country"`
	IsCompany          bool    `gorm:"type:boolean" json:"isCompany"`
}

func (u *UserInfo) GetNameAsString() (string, bool) {
	empty := ""
	if u.IsCompany {
		if u.CompanyName != nil {
			return *u.CompanyName, true
		} else {
			return empty, false
		}
	} else {
		if u.Name != nil && u.Surname != nil {
			name := *u.Name + " " + *u.Surname
			return name, true
		} else {
			return empty, false
		}
	}
}
