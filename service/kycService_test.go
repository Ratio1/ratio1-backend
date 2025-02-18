package service

import (
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/stretchr/testify/require"
)

func Test_CreateKycTable(t *testing.T) {
	//Modify model.Account struct - remove gorm:- before running this

	db, err := storage.GetDB()
	if err != nil {
		panic(err)
	}

	//make query that gets all accounts from db
	selectAllAccountsDbQuery := `select * from eth_accounts;`

	//execute query
	var accounts []model.Account
	db.Raw(selectAllAccountsDbQuery).Scan(&accounts)

	kycs := make([]model.Kyc, 0)

	//iterate over accounts
	for _, account := range accounts {
		if account.EmailConfirmed && account.Email != nil {
			kycs = append(kycs, model.Kyc{
				Email:       *account.Email,
				ApplicantId: "",
				KycStatus:   model.StatusInit,
			})
		}
	}

	//for each chunk of 100 elements in kycs
	for i := 0; i < len(kycs); i += 100 {
		//get chunk
		end := i + 100
		if end > len(kycs) {
			end = len(kycs)
		}

		newKycs := kycs[i:end]
		db.Create(&newKycs)
	}

	var count int64
	db.Model(&model.Kyc{}).Count(&count)
	require.True(t, count == int64(len(kycs)), "bad count")
}
