package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/process"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/storage"
)

/*
..######..########.########..##.....##..######..########..######.......####.......##.....##....###....########...######.
.##....##....##....##.....##.##.....##.##....##....##....##....##.....##..##......##.....##...##.##...##.....##.##....##
.##..........##....##.....##.##.....##.##..........##....##............####.......##.....##..##...##..##.....##.##......
..######.....##....########..##.....##.##..........##.....######......####........##.....##.##.....##.########...######.
.......##....##....##...##...##.....##.##..........##..........##....##..##.##.....##...##..#########.##...##.........##
.##....##....##....##....##..##.....##.##....##....##....##....##....##...##........##.##...##.....##.##....##..##....##
..######.....##....##.....##..#######...######.....##.....######......####..##.......###....##.....##.##.....##..######.
*/

type OfflineNode struct {
	NodeAddress                    string
	NodeAlias                      string
	LastSeenByOraclesWhenEmailSent time.Time
	NotificationTime               time.Time
	NotifiedByNode                 string
}

type RetrievedNodeDetails struct {
	NodeAddress string `json:"eth_addr"`
	Alias       string `json:"alias"`
	LastSeenAgo string `json:"last_seen_ago"`
	IsOnline    bool   `json:"is_online"`
}

type SharedNodeDetails struct {
	NodeEthAddress                 string
	NodeAlias                      string
	LastSeenByOraclesWhenEmailSent string
}

var cstoreKeyForNotifiedNodes = "notified_offline_nodes" //the key in cstore where we will store the list of nodes for whom the email has already been sent (with the day of the last email sent)

/*
.##.....##....###....####.##....##....########.##........#######..##......##
.###...###...##.##....##..###...##....##.......##.......##.....##.##..##..##
.####.####..##...##...##..####..##....##.......##.......##.....##.##..##..##
.##.###.##.##.....##..##..##.##.##....######...##.......##.....##.##..##..##
.##.....##.#########..##..##..####....##.......##.......##.....##.##..##..##
.##.....##.##.....##..##..##...###....##.......##.......##.....##.##..##..##
.##.....##.##.....##.####.##....##....##.......########..#######...###..###.
*/

func FetchOfflineNodesAndSendEmail() {
	ctx := context.Background()
	reportError := newReportError("FetchOfflineNodesAndSendEmail")
	myAddress, err := GetAddress()
	if err != nil {
		reportError("error while retrieving THIS NODE address", err)
		return
	}
	notifiedNodes, err := fetchNotifiedNodesFromCstore(ctx)
	if err != nil {
		reportError("error while fetching notified nodes from cstore", err)
		return
	}

	newNotifiedNodes, nodesToBeNotified, err := fetchOfflineNodes(notifiedNodes, myAddress)
	if err != nil {
		reportError("error while fetching offline nodes", err)
		return
	}

	err = writeOfflineNodesToCstore(ctx, newNotifiedNodes)
	if err != nil {
		reportError("error while writing offline nodes to cstore", err)
		return
	}

	emailsAndNodes, err := retrieveEmailsForOfflineNodes(nodesToBeNotified)
	if err != nil {
		reportError("error while retrieving emails for offline nodes", err)
		return
	}
	err = sendEmailToOwnersOfOfflineNodes(emailsAndNodes)
	if err != nil {
		reportError("error while sending emails to owners of offline nodes", err)
		return
	}
}

/*
.########.##.....##.##....##..######.
.##.......##.....##.###...##.##....##
.##.......##.....##.####..##.##......
.######...##.....##.##.##.##.##......
.##.......##.....##.##..####.##......
.##.......##.....##.##...###.##....##
.##........#######..##....##..######.
*/

func fetchNotifiedNodesFromCstore(ctx context.Context) (map[string]OfflineNode, error) {
	var nodesRetrieved map[string]OfflineNode //k = r1 node address, v = struct with the details of the node (last time email sent, when the node was seen offline, etc)
	_, err := config.Config.CstoreClient.Get(ctx, cstoreKeyForNotifiedNodes, &nodesRetrieved)
	if err != nil {
		return nil, errors.New("error while fetching from cstore the list of nodes:" + err.Error())
	}
	return nodesRetrieved, nil
}

func fetchOfflineNodes(notifiedNodes map[string]OfflineNode, myAddress string) (map[string]OfflineNode, map[string]SharedNodeDetails, error) { //should return a list with all nodes, for each node check: 1) if the node is on the previus list 2) if it's offline for more than x hrs, if the second condition is true, add to a list and return the list.
	NewNotifiedNodes := make(map[string]OfflineNode)        // r1 node address as key
	NodesToBeNotified := make(map[string]SharedNodeDetails) // eth node address as key
	var AllNodes struct {
		Result struct {
			Nodes map[string]json.RawMessage `json:"nodes"` //this is needed because in the returned structure i have nodes : { error : error_string, r1_address: RetrievedNodeDetails,...}
		} `json:"result"`
	}

	//fetch all nodes form api
	url := config.Config.OracleApi + config.Config.OracleNodeListEndpoint
	err := process.HttpGet(url, &AllNodes)
	if err != nil {
		return nil, nil, errors.New("error while calling oracles api:" + err.Error())
	}

	if AllNodes.Result.Nodes["error"] != nil { //the value will be a string, and is always not nil with current API version
		var errorMessage string
		err = json.Unmarshal(AllNodes.Result.Nodes["error"], &errorMessage)
		if err != nil {
			return nil, nil, errors.New("error while unmarshalling error message from oracles api response:" + err.Error())
		}
		if errorMessage != "" {
			return nil, nil, errors.New("oracles api returned an error: " + errorMessage)
		}
	}

	nodes := make(map[string]RetrievedNodeDetails)
	for k, v := range AllNodes.Result.Nodes {
		if k != "error" {
			nodeDetailsBytes, err := json.Marshal(v)
			if err != nil {
				return nil, nil, errors.New("error while marshalling node details for node " + k + ":" + err.Error())
			}
			var nodeDetails RetrievedNodeDetails
			err = json.Unmarshal(nodeDetailsBytes, &nodeDetails)
			if err != nil {
				return nil, nil, errors.New("error while unmarshalling node details for node " + k + ":" + err.Error())
			}
			nodes[k] = nodeDetails
		}
	}

	for r1NodeAddress, nodeDetails := range nodes {
		if nodeDetails.IsOnline { //if the node is online, skip it
			continue
		}

		lastSeenTime, err := retrieveTimeFromLastSeenAgo(nodeDetails.LastSeenAgo)
		if err != nil {
			return nil, nil, errors.New("error while retrieving last seen time for node " + r1NodeAddress + ":" + err.Error())
		}
		if time.Since(lastSeenTime) < 24*time.Hour { //if the node is offline for less than 24 hrs, skip it
			continue
		}
		lastSeenTimeAsString := lastSeenTime.Format("2006-01-02 15:04:05")

		if notifiedNode, ok := notifiedNodes[r1NodeAddress]; ok { //if node was notified
			if time.Since(notifiedNode.NotificationTime) >= 24*time.Hour { //more than 24 hrs ago
				notifiedNode.NotificationTime = time.Now()     //update the notification time
				notifiedNode.NotifiedByNode = myAddress        //update the node that sent the email
				NewNotifiedNodes[r1NodeAddress] = notifiedNode //add to the result list with the same details (last time email sent, when the node was seen offline, etc) (so we will not remove the instance)
				NodesToBeNotified[nodeDetails.NodeAddress] = SharedNodeDetails{
					NodeEthAddress:                 nodeDetails.NodeAddress[:5] + "..." + nodeDetails.NodeAddress[len(nodeDetails.NodeAddress)-5:],
					NodeAlias:                      nodeDetails.Alias,
					LastSeenByOraclesWhenEmailSent: lastSeenTimeAsString,
				}
			} else {
				NewNotifiedNodes[r1NodeAddress] = notifiedNode //add to the result list with the same details (last time email sent, when the node was seen offline, etc) (so we will not remove the instance)
			}
			continue
		}

		NewNotifiedNodes[r1NodeAddress] = OfflineNode{
			NodeAddress:                    r1NodeAddress,
			NodeAlias:                      nodeDetails.Alias,
			LastSeenByOraclesWhenEmailSent: lastSeenTime,
			NotificationTime:               time.Now(),
			NotifiedByNode:                 myAddress,
		}

		NodesToBeNotified[nodeDetails.NodeAddress] = SharedNodeDetails{
			NodeEthAddress:                 nodeDetails.NodeAddress[:5] + "..." + nodeDetails.NodeAddress[len(nodeDetails.NodeAddress)-5:],
			NodeAlias:                      nodeDetails.Alias,
			LastSeenByOraclesWhenEmailSent: lastSeenTimeAsString,
		}
	}
	return NewNotifiedNodes, NodesToBeNotified, nil
}

func writeOfflineNodesToCstore(ctx context.Context, newNotifiedNodes map[string]OfflineNode) error { //write to cstore the list of nodes that you'll send email to.
	err := config.Config.CstoreClient.Set(ctx, cstoreKeyForNotifiedNodes, newNotifiedNodes, nil)
	if err != nil {
		return errors.New("error while writing to cstore the list of nodes:" + err.Error())
	}
	return nil
}

func retrieveEmailsForOfflineNodes(uniqueNodesWithDetails map[string]SharedNodeDetails) (map[string][]SharedNodeDetails, error) { //retrieve the email addresses of the node owners (i cannot know if more nodes are owned by the same person, so i need to retrieve the email for each node), retrun a list of unique addresses
	uniqueEmails := make(map[string][]SharedNodeDetails)
	uniqueOwners := make(map[string][]SharedNodeDetails) //to keep track of unique owners with their offline nodes

	//retrieve unique nodes (no duplicates)
	var uniqueNodes []string
	for node := range uniqueNodesWithDetails {
		uniqueNodes = append(uniqueNodes, node)
	}

	//for each node retrieve the owners
	nodeToOwner, err := getNodeOwners(uniqueNodes)
	if err != nil {
		return nil, errors.New("error while retrieving node owners:" + err.Error())
	}

	//create a map of owner with their nodesDetails
	for node, owner := range nodeToOwner {
		if details, ok := uniqueNodesWithDetails[node]; ok {
			uniqueOwners[owner] = append(uniqueOwners[owner], details)
		} else { //hsould never happen because the nodes in nodeToOwner should be the same as the nodes in uniqueNodesWithDetails, but just in case
			return nil, errors.New("node " + node + " not found in the list of unique nodes with details")
		}
	}

	//for each owner retrieve the email and connect email -> nodeDetails using a map
	for owner, details := range uniqueOwners {
		account, found, err := storage.GetAccountByAddress(owner)
		if err != nil {
			return nil, errors.New("error while retrieving account information for owner " + owner + ":" + err.Error())
		} else if !found {
			return nil, errors.New("account not found for owner " + owner)
		}
		if account.Email != nil && *account.Email != "" {
			uniqueEmails[*account.Email] = append(uniqueEmails[*account.Email], details...) //add all the nodes+details of the same owner to the same email address
		} else if _, ok := config.Config.OwnersToSkipInOfflineNodes[owner]; !ok { //OwnersToSkipInOfflineNodes is a set of known address with "permanent" offline nodes
			notifyError("email not found for owner "+owner, nil)
		}
	}
	return uniqueEmails, nil
}

func sendEmailToOwnersOfOfflineNodes(emailsAndNodes map[string][]SharedNodeDetails) error { //send email to the list of unique addresses retrieved in the previous function.
	for email, nodes := range emailsAndNodes {
		EnqueueEmailTask(NewSendNodesOfflineEmailTask(email, nodes), true)
	}
	return nil
}

/*
.##.....##.########.####.##........######.
.##.....##....##.....##..##.......##....##
.##.....##....##.....##..##.......##......
.##.....##....##.....##..##........######.
.##.....##....##.....##..##.............##
.##.....##....##.....##..##.......##....##
..#######.....##....####.########..######.
*/

func retrieveTimeFromLastSeenAgo(lastSeenAgo string) (time.Time, error) {
	parts := strings.Split(lastSeenAgo, ":")
	if len(parts) != 3 {
		return time.Time{}, errors.New("invalid format for last seen ago")
	}
	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	seconds, _ := strconv.Atoi(parts[2])
	//standard time is 2006-01-02 15:04:05, so we can use it as reference to retrieve the time from the last seen ago
	return time.Now().Add(-time.Duration(hours)*time.Hour - time.Duration(minutes)*time.Minute - time.Duration(seconds)*time.Second), nil
}
