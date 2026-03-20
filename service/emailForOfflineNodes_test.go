package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/Ratio1/edge_sdk_go/pkg/cstore"
	"github.com/stretchr/testify/require"
)

func Test_fetchOfflineNodes(t *testing.T) {
	config.Config.OracleApi = "https://oracle.ratio1.ai"
	config.Config.OracleNodeListEndpoint = "/nodes_list"
	emptyNotifiedNodes := make(map[string]OfflineNode)
	offlineNodes, toBeNotified, err := fetchOfflineNodes(emptyNotifiedNodes, "MyAddress_test")
	require.NoError(t, err)
	writeJSON := func(filename string, v any) {
		t.Helper()

		data, err := json.MarshalIndent(v, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(filename, data, 0o644)
		require.NoError(t, err)
	}

	writeJSON("offlineNodes.json", offlineNodes)
	writeJSON("toBeNotified.json", toBeNotified)

	fmt.Printf("offlineNodes: %v\n", offlineNodes)
	fmt.Printf("toBeNotified: %v\n", toBeNotified)
}

func Test_writeOfflineNodesToCstore(t *testing.T) {
	var err error
	config.Config.CstoreClient, err = cstore.New("http://localhost:8787")
	require.NoError(t, err)
	config.Config.OracleApi = "https://oracle.ratio1.ai"
	config.Config.OracleNodeListEndpoint = "/nodes_list"
	emptyNotifiedNodes := make(map[string]OfflineNode)
	offlineNodes, _, err := fetchOfflineNodes(emptyNotifiedNodes, "MyAddress_test")
	require.NoError(t, err)
	err = writeOfflineNodesToCstore(context.Background(), offlineNodes)
	require.NoError(t, err)
}

func Test_readOfflineNodesFromCstore(t *testing.T) {
	var err error
	config.Config.CstoreClient, err = cstore.New("http://localhost:8787")
	require.NoError(t, err)
	notifiedNodes, err := fetchNotifiedNodesFromCstore(context.Background())
	require.NoError(t, err)
	writeJSON := func(filename string, v any) {
		t.Helper()

		data, err := json.MarshalIndent(v, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(filename, data, 0o644)
		require.NoError(t, err)
	}
	writeJSON("notifiedNodes.json", notifiedNodes)
}

func Test_readFromCstoreAndFetchFromAPI(t *testing.T) {
	var err error
	config.Config.OracleApi = "https://oracle.ratio1.ai"
	config.Config.OracleNodeListEndpoint = "/nodes_list"
	config.Config.CstoreClient, err = cstore.New("http://localhost:8787")
	require.NoError(t, err)
	//fetch all nodes
	emptyNotifiedNodes := make(map[string]OfflineNode)
	offlineNodes, _, err := fetchOfflineNodes(emptyNotifiedNodes, "MyAddress_test")
	require.NoError(t, err)
	//populate cstore
	err = writeOfflineNodesToCstore(context.Background(), offlineNodes)
	require.NoError(t, err)

	//fetch notified nodes from cstore
	notifiedNodes, err := fetchNotifiedNodesFromCstore(context.Background())
	require.NoError(t, err)

	//fetch offline nodes again, this time with the notified nodes from cstore
	_, nodesToBeNotified, err := fetchOfflineNodes(notifiedNodes, "MyAddress_test")
	require.NoError(t, err)

	require.Equal(t, 0, len(nodesToBeNotified))
}

func Test_trySeeNotificationsAfter24Hours(t *testing.T) {
	var err error
	config.Config.OracleApi = "https://oracle.ratio1.ai"
	config.Config.OracleNodeListEndpoint = "/nodes_list"
	config.Config.CstoreClient, err = cstore.New("http://localhost:8787")
	require.NoError(t, err)
	//fetch all nodes
	emptyNotifiedNodes := make(map[string]OfflineNode)
	offlineNodes, _, err := fetchOfflineNodes(emptyNotifiedNodes, "MyAddress_test")
	require.NoError(t, err)

	for k, v := range offlineNodes {
		v.NotificationTime = v.NotificationTime.Add(-25 * time.Hour)
		offlineNodes[k] = v
	}
	//populate cstore
	err = writeOfflineNodesToCstore(context.Background(), offlineNodes)
	require.NoError(t, err)

	//fetch notified nodes from cstore
	notifiedNodes, err := fetchNotifiedNodesFromCstore(context.Background())
	require.NoError(t, err)

	//fetch offline nodes again, this time with the notified nodes from cstore
	_, nodesToBeNotified, err := fetchOfflineNodes(notifiedNodes, "MyAddress_test")
	require.NoError(t, err)

	require.Equal(t, len(offlineNodes), len(nodesToBeNotified))
}

func Test_retrieveEmailForOfflineNodes(t *testing.T) { //! CAUTION: THIS TEST REQUIRE A DB CONNECTION AND INFURA API KEY, USE IT ONLY FOR DEBUGGING PURPOSES
	config.Config.OracleApi = "https://oracle.ratio1.ai"
	config.Config.OracleNodeListEndpoint = "/nodes_list"
	emptyNotifiedNodes := make(map[string]OfflineNode)
	_, toBeNotified, err := fetchOfflineNodes(emptyNotifiedNodes, "MyAddress_test")
	require.NoError(t, err)

	emailsAndNodes, err := retrieveEmailsForOfflineNodes(toBeNotified)
	require.NoError(t, err)
	writeJSON := func(filename string, v any) {
		t.Helper()

		data, err := json.MarshalIndent(v, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(filename, data, 0o644)
		require.NoError(t, err)
	}
	writeJSON("emailsAndNodes.json", emailsAndNodes)
}
