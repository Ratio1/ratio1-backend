package service

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

const maxLogFilterBlockRange int64 = 10_000
const maxLogFilterResults = 10_000
const logFilterMaxAttempts = 3
const logFilterRetryDelay = 250 * time.Millisecond

type logFilterer interface {
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)
}

func filterLogsInChunks(ctx context.Context, client logFilterer, query ethereum.FilterQuery, from, to int64) ([]types.Log, error) {
	if from < 0 || to < 0 {
		return nil, fmt.Errorf("block range must be non-negative: %d-%d", from, to)
	}
	if from > to {
		return []types.Log{}, nil
	}

	logs := make([]types.Log, 0)
	for chunkStart := from; chunkStart <= to; {
		chunkEnd := to
		if to-chunkStart >= maxLogFilterBlockRange {
			chunkEnd = chunkStart + maxLogFilterBlockRange - 1
		}

		chunkLogs, err := filterLogsChunk(ctx, client, query, chunkStart, chunkEnd)
		if err != nil {
			return nil, err
		}
		logs = append(logs, chunkLogs...)

		if chunkEnd == to {
			break
		}
		chunkStart = chunkEnd + 1
	}

	return logs, nil
}

func filterLogsChunk(ctx context.Context, client logFilterer, query ethereum.FilterQuery, from, to int64) ([]types.Log, error) {
	chunkQuery := query
	chunkQuery.FromBlock = big.NewInt(from)
	chunkQuery.ToBlock = big.NewInt(to)

	logs, err := filterLogsWithRetry(ctx, client, chunkQuery)
	if err != nil {
		return nil, fmt.Errorf("filtering logs for blocks %d-%d: %w", from, to, err)
	}
	if len(logs) < maxLogFilterResults {
		return logs, nil
	}
	if from == to {
		return nil, fmt.Errorf("filtering logs for block %d returned %d results; completeness cannot be guaranteed", from, len(logs))
	}

	middle := from + (to-from)/2
	leftLogs, err := filterLogsChunk(ctx, client, query, from, middle)
	if err != nil {
		return nil, err
	}
	rightLogs, err := filterLogsChunk(ctx, client, query, middle+1, to)
	if err != nil {
		return nil, err
	}

	return append(leftLogs, rightLogs...), nil
}

func filterLogsWithRetry(ctx context.Context, client logFilterer, query ethereum.FilterQuery) ([]types.Log, error) {
	var err error
	for attempt := 1; attempt <= logFilterMaxAttempts; attempt++ {
		var logs []types.Log
		logs, err = client.FilterLogs(ctx, query)
		if err == nil {
			return logs, nil
		}
		if attempt == logFilterMaxAttempts {
			break
		}

		delay := logFilterRetryDelay * time.Duration(1<<(attempt-1))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, err
}
