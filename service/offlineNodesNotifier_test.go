package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/stretchr/testify/require"
)

func boolPtr(value bool) *bool {
	return &value
}

type mockOwnerNotificationStore struct {
	syncErr       error
	lastSentByKey map[string]time.Time
	lastSentErr   error
	setErr        error
	setCalls      int
}

func (m *mockOwnerNotificationStore) Sync(ctx context.Context) error {
	return m.syncErr
}

func (m *mockOwnerNotificationStore) LastSent(ctx context.Context, ownerAddress string) (time.Time, bool, error) {
	if m.lastSentErr != nil {
		return time.Time{}, false, m.lastSentErr
	}
	value, found := m.lastSentByKey[ownerAddress]
	return value, found, nil
}

func (m *mockOwnerNotificationStore) SetLastSent(ctx context.Context, ownerAddress string, sentAt time.Time) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.setCalls++
	if m.lastSentByKey == nil {
		m.lastSentByKey = make(map[string]time.Time)
	}
	m.lastSentByKey[ownerAddress] = sentAt
	return nil
}

func TestFetchOracleNodesListFiltersOfflineNodes(t *testing.T) {
	previousHTTPGet := oracleHTTPGetFn
	defer func() {
		oracleHTTPGetFn = previousHTTPGet
	}()

	oracleHTTPGetFn = func(url string, castTarget interface{}, headers ...process.HttpHeaderPair) error {
		require.Equal(t, "https://oracle.test/nodes_list", url)
		response := castTarget.(*oracleNodesListResponse)
		response.Result.Nodes = map[string]oracleNodeRaw{
			"node1": {Alias: "alpha", EthAddr: "0x1111111111111111111111111111111111111111", IsOnline: boolPtr(false), LastState: "2026-06-08 10:00:00", LastSeenAgo: "25:01:01"},
			"node2": {Alias: "beta", EthAddr: "0x2222222222222222222222222222222222222222", IsOnline: boolPtr(false), LastState: "2026-06-08 12:00:00", LastSeenAgo: "24:00:00"},
			"node3": {Alias: "gamma", EthAddr: "0x3333333333333333333333333333333333333333", IsOnline: boolPtr(true), LastState: "2026-06-09 10:00:00", LastSeenAgo: 99999},
			"node4": {Alias: "delta", EthAddr: "0x4444444444444444444444444444444444444444", LastSeenAgo: 92000},
		}
		return nil
	}

	oldOraclesAPI := config.Config.OraclesApi
	config.Config.OraclesApi = "https://oracle.test"
	defer func() {
		config.Config.OraclesApi = oldOraclesAPI
	}()

	nodes, err := fetchOracleNodesList()
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "alpha", nodes[0].NodeAlias)
	require.Equal(t, "0x1111111111111111111111111111111111111111", nodes[0].NodeAddress)
	require.True(t, nodes[0].OfflineSeconds > int64(offlineNodeThreshold.Seconds()))
}

func TestValidateOfflineNodesNotifierConfigRequiresCStore(t *testing.T) {
	previousNewCStoreClient := newCStoreClientFromEnvFn
	defer func() {
		newCStoreClientFromEnvFn = previousNewCStoreClient
	}()

	oldOraclesAPI := config.Config.OraclesApi
	config.Config.OraclesApi = "https://oracle.test"
	defer func() {
		config.Config.OraclesApi = oldOraclesAPI
	}()

	t.Setenv("EE_CHAINSTORE_API_URL", "")
	err := ValidateOfflineNodesNotifierConfig()
	require.ErrorContains(t, err, "EE_CHAINSTORE_API_URL is not set")
}

func TestNotifyOfflineLinkedNodesFailClosedWhenStoreInitFails(t *testing.T) {
	previousFetch := fetchOracleNodesListFn
	previousStore := newOwnerNotificationStoreFn
	previousResolve := resolveNodeOwnersFn
	previousGetAccount := getAccountByAddressFn
	previousGetNotificationEmail := getNotificationEmailFn
	previousSend := sendOfflineNodesEmailFn
	defer func() {
		fetchOracleNodesListFn = previousFetch
		newOwnerNotificationStoreFn = previousStore
		resolveNodeOwnersFn = previousResolve
		getAccountByAddressFn = previousGetAccount
		getNotificationEmailFn = previousGetNotificationEmail
		sendOfflineNodesEmailFn = previousSend
	}()

	fetchOracleNodesListFn = func() ([]OfflineNodeAlert, error) {
		return []OfflineNodeAlert{
			{NodeAlias: "alpha", NodeAddress: "0x1111111111111111111111111111111111111111", OfflineSeconds: 100000},
		}, nil
	}
	newOwnerNotificationStoreFn = func() (ownerNotificationStore, error) {
		return nil, errors.New("store unavailable")
	}

	resolveCalled := false
	resolveNodeOwnersFn = func(nodes []string) (map[string]string, error) {
		resolveCalled = true
		return map[string]string{}, nil
	}

	sendCalled := false
	sendOfflineNodesEmailFn = func(email string, nodes []OfflineNodeAlert) error {
		sendCalled = true
		return nil
	}

	NotifyOfflineLinkedNodes()

	require.False(t, resolveCalled)
	require.False(t, sendCalled)
}

func TestNotifyOfflineLinkedNodesGroupsAndThrottlesOwners(t *testing.T) {
	previousFetch := fetchOracleNodesListFn
	previousStore := newOwnerNotificationStoreFn
	previousResolve := resolveNodeOwnersFn
	previousGetAccount := getAccountByAddressFn
	previousGetNotificationEmail := getNotificationEmailFn
	previousSend := sendOfflineNodesEmailFn
	defer func() {
		fetchOracleNodesListFn = previousFetch
		newOwnerNotificationStoreFn = previousStore
		resolveNodeOwnersFn = previousResolve
		getAccountByAddressFn = previousGetAccount
		getNotificationEmailFn = previousGetNotificationEmail
		sendOfflineNodesEmailFn = previousSend
	}()

	fetchOracleNodesListFn = func() ([]OfflineNodeAlert, error) {
		return []OfflineNodeAlert{
			{NodeAlias: "alpha", NodeAddress: "0x1111111111111111111111111111111111111111", OfflineSeconds: 100000},
			{NodeAlias: "beta", NodeAddress: "0x2222222222222222222222222222222222222222", OfflineSeconds: 110000},
			{NodeAlias: "gamma", NodeAddress: "0x3333333333333333333333333333333333333333", OfflineSeconds: 120000},
		}, nil
	}

	mockStore := &mockOwnerNotificationStore{
		lastSentByKey: map[string]time.Time{
			"0xowner2": time.Now().UTC().Add(-2 * time.Hour),
		},
	}
	newOwnerNotificationStoreFn = func() (ownerNotificationStore, error) {
		return mockStore, nil
	}

	resolvedAddresses := make([]string, 0)
	resolveNodeOwnersFn = func(nodes []string) (map[string]string, error) {
		resolvedAddresses = append(resolvedAddresses, nodes...)
		return map[string]string{
			"0x1111111111111111111111111111111111111111": "0xowner1",
			"0x2222222222222222222222222222222222222222": "0xowner1",
			"0x3333333333333333333333333333333333333333": "0xowner2",
		}, nil
	}

	getAccountByAddressFn = func(address string) (*model.Account, bool, error) {
		switch address {
		case "0xowner1":
			email := "owner1@example.com"
			return &model.Account{
				Address:        address,
				Email:          &email,
				EmailConfirmed: true,
			}, true, nil
		case "0xowner2":
			email := "owner2@example.com"
			return &model.Account{
				Address:        address,
				Email:          &email,
				EmailConfirmed: true,
			}, true, nil
		default:
			return nil, false, nil
		}
	}

	getNotificationEmailFn = func(address string) (*model.AccountNotificationEmail, bool, error) {
		if address != "0xowner1" {
			return nil, false, nil
		}
		email := "alerts@example.com"
		return &model.AccountNotificationEmail{
			AccountAddress: address,
			Email:          &email,
			EmailConfirmed: true,
		}, true, nil
	}

	sendCalls := 0
	var sentEmails []string
	var sentNodes []OfflineNodeAlert
	sendOfflineNodesEmailFn = func(email string, nodes []OfflineNodeAlert) error {
		sendCalls++
		sentEmails = append(sentEmails, email)
		sentNodes = append([]OfflineNodeAlert(nil), nodes...)
		return nil
	}

	NotifyOfflineLinkedNodes()

	require.Len(t, resolvedAddresses, 3)
	require.Equal(t, 2, sendCalls)
	require.ElementsMatch(t, []string{"owner1@example.com", "alerts@example.com"}, sentEmails)
	require.Len(t, sentNodes, 2)
	require.Equal(t, 1, mockStore.setCalls)
}

func TestNotifyOfflineLinkedNodesSkipsSetWhenEmailFails(t *testing.T) {
	previousFetch := fetchOracleNodesListFn
	previousStore := newOwnerNotificationStoreFn
	previousResolve := resolveNodeOwnersFn
	previousGetAccount := getAccountByAddressFn
	previousGetNotificationEmail := getNotificationEmailFn
	previousSend := sendOfflineNodesEmailFn
	defer func() {
		fetchOracleNodesListFn = previousFetch
		newOwnerNotificationStoreFn = previousStore
		resolveNodeOwnersFn = previousResolve
		getAccountByAddressFn = previousGetAccount
		getNotificationEmailFn = previousGetNotificationEmail
		sendOfflineNodesEmailFn = previousSend
	}()

	fetchOracleNodesListFn = func() ([]OfflineNodeAlert, error) {
		return []OfflineNodeAlert{
			{NodeAlias: "alpha", NodeAddress: "0x1111111111111111111111111111111111111111", OfflineSeconds: 100000},
		}, nil
	}

	mockStore := &mockOwnerNotificationStore{
		lastSentByKey: map[string]time.Time{},
	}
	newOwnerNotificationStoreFn = func() (ownerNotificationStore, error) {
		return mockStore, nil
	}

	resolveNodeOwnersFn = func(nodes []string) (map[string]string, error) {
		return map[string]string{
			"0x1111111111111111111111111111111111111111": "0xowner1",
		}, nil
	}

	getAccountByAddressFn = func(address string) (*model.Account, bool, error) {
		email := "owner1@example.com"
		return &model.Account{
			Address:        address,
			Email:          &email,
			EmailConfirmed: true,
		}, true, nil
	}

	sendOfflineNodesEmailFn = func(email string, nodes []OfflineNodeAlert) error {
		return errors.New("email send failed")
	}

	NotifyOfflineLinkedNodes()

	require.Equal(t, 0, mockStore.setCalls)
}
