package service

import (
	"strings"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequireApprovedKyc_ShouldOnlyAdmitActiveApprovedKyc(t *testing.T) {
	requireServiceTestDatabase(t)
	db, err := storage.GetDB()
	require.NoError(t, err)

	cases := []struct {
		name        string
		withEmail   bool
		blacklisted bool
		kyc         *model.Kyc
		admitted    bool
	}{
		{name: "approved", withEmail: true, kyc: &model.Kyc{KycStatus: model.StatusApproved, IsActive: true}, admitted: true},
		{name: "no email", kyc: nil},
		{name: "email without kyc", withEmail: true},
		{name: "pending", withEmail: true, kyc: &model.Kyc{KycStatus: "pending", IsActive: true}},
		{name: "inactive", withEmail: true, kyc: &model.Kyc{KycStatus: model.StatusApproved, IsActive: false}},
		{name: "deleted", withEmail: true, kyc: &model.Kyc{KycStatus: model.StatusApproved, IsActive: true, HasBeenDeleted: true}},
		{name: "blacklisted", withEmail: true, blacklisted: true, kyc: &model.Kyc{KycStatus: model.StatusApproved, IsActive: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			address := newTestWalletAddress(t)
			account := &model.Account{Address: address, IsBlacklisted: tc.blacklisted, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			email := strings.ToLower(uuid.NewString()) + "@example.test"
			if tc.withEmail {
				account.Email = &email
			}
			require.NoError(t, storage.CreateAccount(account))
			t.Cleanup(func() { require.NoError(t, db.Where("address = ?", address).Delete(&model.Account{}).Error) })
			if tc.kyc != nil {
				tc.kyc.Uuid, tc.kyc.Email, tc.kyc.ApplicantType, tc.kyc.LastUpdated = uuid.New(), email, model.IndividualCustomer, time.Now()
				// GORM inserts the DB default (true) for a false IsActive and writes it
				// back into the struct, so keep the intended value and set it afterwards.
				isActive := tc.kyc.IsActive
				require.NoError(t, db.Create(tc.kyc).Error)
				require.NoError(t, db.Model(&model.Kyc{}).Where("uuid = ?", tc.kyc.Uuid).Update("is_active", isActive).Error)
				t.Cleanup(func() { require.NoError(t, db.Where("email = ?", email).Delete(&model.Kyc{}).Error) })
			}

			err := RequireApprovedKyc(address)
			if tc.admitted {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrorKycNotApproved)
			}
		})
	}
}

func TestRequireApprovedKyc_ShouldNotCreateAccountForUnknownWallet(t *testing.T) {
	requireServiceTestDatabase(t)
	before := countAccounts(t)

	require.ErrorIs(t, RequireApprovedKyc(newTestWalletAddress(t)), ErrorKycNotApproved)
	require.Equal(t, before, countAccounts(t))
}
