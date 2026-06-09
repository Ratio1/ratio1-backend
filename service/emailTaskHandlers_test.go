package service

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestSendJobsEndingEmailTaskPayloadRoundTrip(t *testing.T) {
	largeJobID, ok := new(big.Int).SetString("123456789012345678901234567890", 10)
	if !ok {
		t.Fatal("failed to build test job id")
	}

	task := NewSendJobsEndingEmailTask("owner@example.com", []EndingJob{
		{
			JobID:              largeJobID,
			PricePerEpoch:      big.NewInt(42),
			EscrowOwner:        common.HexToAddress("0x1111111111111111111111111111111111111111"),
			EscrowAddress:      common.HexToAddress("0x2222222222222222222222222222222222222222"),
			NotifyBeforeEpochs: 3,
			JobName:            "critical job",
		},
	})

	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}

	var decodedTask EmailTask
	if err := json.Unmarshal(raw, &decodedTask); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}

	var payload sendJobsEndingEmailPayload
	if err := decodeEmailTaskPayload(decodedTask, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if payload.Recipient != "owner@example.com" {
		t.Fatalf("unexpected recipient: %s", payload.Recipient)
	}
	if len(payload.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(payload.Jobs))
	}
	if payload.Jobs[0].JobID.Cmp(largeJobID) != 0 {
		t.Fatalf("job id lost precision: got %s want %s", payload.Jobs[0].JobID, largeJobID)
	}
	if payload.Jobs[0].EscrowOwner.Hex() != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("unexpected escrow owner: %s", payload.Jobs[0].EscrowOwner.Hex())
	}
}
