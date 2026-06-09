package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
	"github.com/Ratio1/edge_sdk_go/pkg/cstore"
)

const (
	offlineNodeThreshold        = 24 * time.Hour
	ownerNotificationInterval   = 24 * time.Hour
	offlineNodesCStoreHashKey   = "ratio1:backend:offline_nodes_notifier:v1"
	offlineNotifierStoreTimeout = 30 * time.Second
)

var (
	fetchOracleNodesListFn      = fetchOracleNodesList
	resolveNodeOwnersFn         = getNodeOwners
	getAccountByAddressFn       = storage.GetAccountByAddress
	getNotificationEmailFn      = storage.GetAccountNotificationEmailByAddress
	sendOfflineNodesEmailFn     = SendOfflineNodesEmail
	newCStoreClientFromEnvFn    = cstore.NewFromEnv
	newOwnerNotificationStoreFn = newCStoreOwnerNotificationStore
	oracleHTTPGetFn             = process.HttpGet
)

type OfflineNodeAlert struct {
	NodeAlias      string
	NodeAddress    string
	OfflineSeconds int64
}

func ValidateOfflineNodesNotifierConfig() error {
	if strings.TrimSpace(config.Config.OraclesApi) == "" {
		return errors.New("oracles api url is not configured")
	}
	if strings.TrimSpace(os.Getenv("EE_CHAINSTORE_API_URL")) == "" {
		return errors.New("EE_CHAINSTORE_API_URL is not set")
	}
	if _, err := newCStoreClientFromEnvFn(); err != nil {
		return errors.New("invalid CStore configuration: " + err.Error())
	}
	return nil
}

type oracleNodesListResponse struct {
	Result oracleNodesListResult `json:"result"`
}

type oracleNodesListResult struct {
	Nodes map[string]oracleNodeRaw `json:"nodes"`
	Error any                      `json:"error"`
}

type oracleNodeRaw struct {
	Alias        string `json:"alias"`
	EthAddress   string `json:"eth_address"`
	EthAddr      string `json:"eth_addr"`
	LastState    string `json:"last_state"`
	LastSeenAgo  any    `json:"last_seen_ago"`
	NodeIsOnline *bool  `json:"node_is_online"`
	IsOnline     *bool  `json:"is_online"`
}

type ownerNotificationStore interface {
	Sync(ctx context.Context) error
	LastSent(ctx context.Context, ownerAddress string) (time.Time, bool, error)
	SetLastSent(ctx context.Context, ownerAddress string, sentAt time.Time) error
}

type cstoreOwnerNotificationStore struct {
	client *cstore.Client
}

type cstoreOwnerLastSent struct {
	LastSentUnix int64 `json:"last_sent_unix"`
}

func NotifyOfflineLinkedNodes() {
	offlineNodes, err := fetchOracleNodesListFn()
	if err != nil {
		log.Error("offline nodes notifier failed to fetch oracle nodes list: %s", err.Error())
		return
	}

	if len(offlineNodes) == 0 {
		return
	}

	store, err := newOwnerNotificationStoreFn()
	if err != nil {
		log.Error("offline nodes notifier failed to initialize cstore: %s", err.Error())
		return
	}

	syncCtx, cancelSync := context.WithTimeout(context.Background(), offlineNotifierStoreTimeout)
	err = store.Sync(syncCtx)
	cancelSync()
	if err != nil {
		log.Error("offline nodes notifier failed to sync cstore state: %s", err.Error())
		return
	}

	nodeAddresses := make([]string, 0, len(offlineNodes))
	nodeByAddress := make(map[string]OfflineNodeAlert, len(offlineNodes))
	for _, node := range offlineNodes {
		address := strings.ToLower(strings.TrimSpace(node.NodeAddress))
		if address == "" {
			continue
		}
		nodeAddresses = append(nodeAddresses, address)
		nodeByAddress[address] = node
	}
	if len(nodeAddresses) == 0 {
		return
	}

	ownersByNode, err := resolveNodeOwnersFn(nodeAddresses)
	if err != nil {
		log.Error("offline nodes notifier failed to resolve node owners: %s", err.Error())
		return
	}

	nodesByOwner := make(map[string][]OfflineNodeAlert)
	for nodeAddress, ownerAddress := range ownersByNode {
		owner := strings.TrimSpace(ownerAddress)
		if owner == "" || strings.EqualFold(owner, "0x0000000000000000000000000000000000000000") {
			continue
		}
		node, found := nodeByAddress[strings.ToLower(strings.TrimSpace(nodeAddress))]
		if !found {
			continue
		}
		nodesByOwner[owner] = append(nodesByOwner[owner], node)
	}

	now := time.Now().UTC()
	for ownerAddress, ownerNodes := range nodesByOwner {
		emails, err := getConfirmedAccountEmails(ownerAddress)
		if err != nil {
			log.Error("offline nodes notifier failed account lookup for %s: %s", ownerAddress, err.Error())
			continue
		}
		if len(emails) == 0 {
			continue
		}

		sort.Slice(ownerNodes, func(i, j int) bool {
			left := ownerNodes[i]
			right := ownerNodes[j]
			if left.OfflineSeconds != right.OfflineSeconds {
				return left.OfflineSeconds > right.OfflineSeconds
			}
			return strings.Compare(left.NodeAddress, right.NodeAddress) < 0
		})

		lastSentCtx, cancelLastSent := context.WithTimeout(context.Background(), offlineNotifierStoreTimeout)
		lastSent, hasLastSent, err := store.LastSent(lastSentCtx, strings.ToLower(ownerAddress))
		cancelLastSent()
		if err != nil {
			log.Error("offline nodes notifier failed cstore read for %s: %s", ownerAddress, err.Error())
			return
		}

		if hasLastSent && now.Sub(lastSent) < ownerNotificationInterval {
			continue
		}

		sentCount := 0
		for _, email := range emails {
			err = sendOfflineNodesEmailFn(email, ownerNodes)
			if err != nil {
				log.Error("offline nodes notifier failed to send email to %s: %s", email, err.Error())
				continue
			}
			sentCount++
		}
		if sentCount == 0 {
			continue
		}

		setLastSentCtx, cancelSetLastSent := context.WithTimeout(context.Background(), offlineNotifierStoreTimeout)
		err = store.SetLastSent(setLastSentCtx, strings.ToLower(ownerAddress), now)
		cancelSetLastSent()
		if err != nil {
			log.Error("offline nodes notifier failed cstore write for %s: %s", ownerAddress, err.Error())
			return
		}
	}
}

func fetchOracleNodesList() ([]OfflineNodeAlert, error) {
	url := strings.TrimSuffix(strings.TrimSpace(config.Config.OraclesApi), "/") + "/nodes_list"
	if strings.TrimSpace(config.Config.OraclesApi) == "" {
		return nil, errors.New("oracles api url is not configured")
	}

	var response oracleNodesListResponse
	err := oracleHTTPGetFn(url, &response)
	if err != nil {
		return nil, errors.New("error while requesting nodes list: " + err.Error())
	}

	if response.Result.Error != nil {
		errMsg := strings.TrimSpace(fmt.Sprintf("%v", response.Result.Error))
		if errMsg != "" && errMsg != "<nil>" {
			return nil, errors.New("oracles api returned error: " + errMsg)
		}
	}

	if len(response.Result.Nodes) == 0 {
		return nil, nil
	}

	offlineNodes := make([]OfflineNodeAlert, 0)
	for _, node := range response.Result.Nodes {
		ethAddress := strings.TrimSpace(node.EthAddress)
		if ethAddress == "" {
			ethAddress = strings.TrimSpace(node.EthAddr)
		}
		if ethAddress == "" {
			continue
		}

		isOffline, hasState := parseNodeOfflineState(node)
		if !hasState || !isOffline {
			continue
		}

		lastSeenSeconds, ok := parseLastSeenSeconds(node.LastSeenAgo)
		if !ok {
			continue
		}
		if lastSeenSeconds <= int64(offlineNodeThreshold.Seconds()) {
			continue
		}

		alias := strings.TrimSpace(node.Alias)
		if alias == "" {
			alias = "unknown"
		}

		offlineNodes = append(offlineNodes, OfflineNodeAlert{
			NodeAlias:      alias,
			NodeAddress:    ethAddress,
			OfflineSeconds: lastSeenSeconds,
		})
	}

	return offlineNodes, nil
}

func parseNodeOfflineState(node oracleNodeRaw) (bool, bool) {
	if node.NodeIsOnline != nil {
		return !(*node.NodeIsOnline), true
	}
	if node.IsOnline != nil {
		return !(*node.IsOnline), true
	}
	state := strings.TrimSpace(strings.ToLower(node.LastState))
	switch state {
	case "offline":
		return true, true
	case "online":
		return false, true
	default:
		return false, false
	}
}

func parseLastSeenSeconds(raw any) (int64, bool) {
	switch v := raw.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return int64(v), true
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return 0, false
		}
		return int64(v), true
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	case string:
		return parseLastSeenSecondsFromString(v)
	default:
		return 0, false
	}
}

func parseLastSeenSecondsFromString(raw string) (int64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false
	}

	if numericValue, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(numericValue), true
	}

	if durationValue, err := time.ParseDuration(s); err == nil {
		return int64(durationValue.Seconds()), true
	}

	if strings.Count(s, ":") == 2 || strings.Count(s, ":") == 1 {
		parts := strings.Split(s, ":")
		total := int64(0)
		multiplier := int64(1)
		for i := len(parts) - 1; i >= 0; i-- {
			part := strings.TrimSpace(parts[i])
			component, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return 0, false
			}
			total += component * multiplier
			multiplier *= 60
		}
		return total, true
	}

	return 0, false
}

func getConfirmedAccountEmails(ownerAddress string) ([]string, error) {
	address := strings.TrimSpace(ownerAddress)
	if address == "" {
		return nil, nil
	}

	account, found, err := getAccountByAddressFn(address)
	if err != nil {
		return nil, err
	}
	if !found && strings.ToLower(address) != address {
		account, found, err = getAccountByAddressFn(strings.ToLower(address))
		if err != nil {
			return nil, err
		}
	}
	if !found || account == nil {
		return nil, nil
	}

	notificationEmail, found, err := getNotificationEmailFn(account.Address)
	if err != nil {
		return nil, err
	}
	if !found {
		notificationEmail = nil
	}

	return notificationEmailsForAccount(account, notificationEmail), nil
}

func newCStoreOwnerNotificationStore() (ownerNotificationStore, error) {
	client, err := newCStoreClientFromEnvFn()
	if err != nil {
		return nil, err
	}
	return &cstoreOwnerNotificationStore{
		client: client,
	}, nil
}

func (s *cstoreOwnerNotificationStore) Sync(ctx context.Context) error {
	_, err := s.client.GetStatus(ctx)
	return err
}

func (s *cstoreOwnerNotificationStore) LastSent(ctx context.Context, ownerAddress string) (time.Time, bool, error) {
	var entry cstoreOwnerLastSent
	item, err := s.client.HGet(ctx, offlineNodesCStoreHashKey, ownerAddress, &entry)
	if err != nil {
		return time.Time{}, false, err
	}
	if item == nil || entry.LastSentUnix <= 0 {
		return time.Time{}, false, nil
	}
	return time.Unix(entry.LastSentUnix, 0).UTC(), true, nil
}

func (s *cstoreOwnerNotificationStore) SetLastSent(ctx context.Context, ownerAddress string, sentAt time.Time) error {
	return s.client.HSet(ctx, offlineNodesCStoreHashKey, ownerAddress, cstoreOwnerLastSent{
		LastSentUnix: sentAt.UTC().Unix(),
	}, nil)
}
