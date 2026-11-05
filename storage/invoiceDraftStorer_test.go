package storage

import (
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInvoiceDraftOwnership(t *testing.T) {
	requireStorageTestDatabase(t)
	db, err := GetDB()
	require.NoError(t, err)
	owner, csp := uuid.NewString(), uuid.NewString()
	for _, address := range []string{owner, csp} {
		require.NoError(t, db.Create(&model.UserInfo{BlockchainAddress: address, Email: address + "@example.test"}).Error)
	}
	t.Cleanup(func() {
		require.NoError(t, db.Where("blockchain_address IN ?", []string{owner, csp}).Delete(&model.UserInfo{}).Error)
	})
	draft := model.InvoiceDraft{DraftId: uuid.New(), UserAddress: owner, CspOwner: csp, CreationTimestamp: time.Now()}
	require.NoError(t, db.Omit("CspProfile", "UserProfile").Create(&draft).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Delete(&draft).Error)
	})
	for _, test := range []struct {
		name   string
		lookup func(string, string) (*model.InvoiceDraft, error)
		owner  string
		other  string
	}{
		{"node owner", GetDraftByReportId, owner, csp},
		{"CSP owner", GetCspDraftByReportId, csp, owner},
	} {
		t.Run(test.name, func(t *testing.T) {
			found, err := test.lookup(draft.DraftId.String(), test.owner)
			require.NoError(t, err)
			require.Equal(t, draft.DraftId, found.DraftId)
			for _, address := range []string{test.other, uuid.NewString()} {
				found, err = test.lookup(draft.DraftId.String(), address)
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
				require.Nil(t, found)
			}
			found, err = test.lookup(uuid.NewString(), test.owner)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound)
			require.Nil(t, found)
		})
	}
}
