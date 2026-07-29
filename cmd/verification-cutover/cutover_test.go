package main

import (
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/stretchr/testify/require"
)

func TestValidateOptions(t *testing.T) {
	require.NoError(t, validateOptions(cutoverOptions{}))
	require.NoError(t, validateOptions(cutoverOptions{
		ExpectedTotal:          10,
		ExpectedPreserved:      4,
		ExpectedReset:          6,
		ExpectedResetUserInfos: 2,
	}))
	require.NoError(t, validateOptions(cutoverOptions{
		Apply:                  true,
		ExpectedTotal:          10,
		ExpectedPreserved:      4,
		ExpectedReset:          6,
		ExpectedResetUserInfos: 2,
	}))

	require.Error(t, validateOptions(cutoverOptions{
		Apply:                  true,
		ExpectedTotal:          -1,
		ExpectedPreserved:      -1,
		ExpectedReset:          -1,
		ExpectedResetUserInfos: -1,
	}))
	require.Error(t, validateOptions(cutoverOptions{
		ExpectedTotal:          10,
		ExpectedPreserved:      -1,
		ExpectedReset:          -1,
		ExpectedResetUserInfos: -1,
	}))
	require.Error(t, validateOptions(cutoverOptions{
		Apply:                  true,
		ExpectedTotal:          10,
		ExpectedPreserved:      4,
		ExpectedReset:          5,
		ExpectedResetUserInfos: 2,
	}))
}

func TestParseDatabaseLink(t *testing.T) {
	cfg, err := parseDatabaseLink("db.example:5432:ratio1:cutover:password:with:colons", "")
	require.NoError(t, err)
	require.Equal(t, "db.example", cfg.Host)
	require.Equal(t, 5432, cfg.Port)
	require.Equal(t, "ratio1", cfg.Database)
	require.Equal(t, "cutover", cfg.User)
	require.Equal(t, "password:with:colons", cfg.Password)
	require.Equal(t, "require", cfg.SSLMode)

	_, err = parseDatabaseLink("db.example:not-a-port:ratio1:cutover:secret", "")
	require.Error(t, err)
	_, err = parseDatabaseLink("db.example:5432:ratio1:cutover", "")
	require.Error(t, err)
	_, err = parseDatabaseLink("db.example:5432:ratio1:cutover:secret", "disable")
	require.Error(t, err)
}

func TestValidatePreCutoverAggregate(t *testing.T) {
	valid := validPreCutoverAggregate()
	require.NoError(t, validatePreCutoverAggregate(valid, cutoverOptions{
		ExpectedTotal:          4,
		ExpectedPreserved:      2,
		ExpectedReset:          2,
		ExpectedResetUserInfos: 1,
	}))

	tests := []struct {
		name   string
		mutate func(*cutoverAggregate)
	}{
		{
			name: "duplicate email",
			mutate: func(value *cutoverAggregate) {
				value.DuplicateEmailGroups = 1
			},
		},
		{
			name: "invalid identity",
			mutate: func(value *cutoverAggregate) {
				value.InvalidIdentityRows = 1
			},
		},
		{
			name: "unknown status",
			mutate: func(value *cutoverAggregate) {
				value.Statuses["mystery"] = 1
			},
		},
		{
			name: "Didit traffic already exists",
			mutate: func(value *cutoverAggregate) {
				value.Providers = map[string]int64{model.VerificationProviderDidit: 1}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validPreCutoverAggregate()
			test.mutate(&value)
			require.Error(t, validatePreCutoverAggregate(value, cutoverOptions{}))
		})
	}

	require.Error(t, validatePreCutoverAggregate(valid, cutoverOptions{
		ExpectedTotal:          5,
		ExpectedPreserved:      2,
		ExpectedReset:          3,
		ExpectedResetUserInfos: 1,
	}))
}

func TestValidatePostCutoverAggregate(t *testing.T) {
	before := validPreCutoverAggregate()
	after := cutoverAggregate{
		Total:          4,
		Approved:       1,
		FinalRejected:  1,
		Preserved:      2,
		Reset:          2,
		UserInfos:      2,
		ResetUserInfos: 0,
		Statuses: map[string]int64{
			model.StatusAccountCreated: 2,
			model.StatusApproved:       1,
			model.StatusFinalRejected:  1,
		},
		Providers: map[string]int64{
			model.VerificationProviderSumsub: 2,
			model.VerificationProviderDidit:  2,
		},
	}
	require.NoError(t, validatePostCutoverAggregate(before, after))

	after.UserInfos = 3
	require.Error(t, validatePostCutoverAggregate(before, after))
}

func validPreCutoverAggregate() cutoverAggregate {
	return cutoverAggregate{
		Total:                   4,
		Approved:                1,
		FinalRejected:           1,
		Preserved:               2,
		Reset:                   2,
		UserInfos:               3,
		ResetUserInfos:          1,
		ResetRowsWithLegacyData: 2,
		Statuses: map[string]int64{
			model.StatusApproved:      1,
			model.StatusFinalRejected: 1,
			model.StatusPending:       1,
			model.StatusRejected:      1,
		},
		Providers: map[string]int64{"": 4},
	}
}
