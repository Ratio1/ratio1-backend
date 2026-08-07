package service

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

const maxLogFilterBlockRange int64 = 10_000
const maxLogFilterResults = 10_000
const logFilterMaxAttempts = 3
const logFilterRateLimitMaxAttempts = 6
const logFilterRequestInterval = time.Second
const logFilterRetryDelay = 250 * time.Millisecond
const logFilterRateLimitRetryDelay = 2 * time.Second

type logFilterWaitFunc func(context.Context, time.Duration) error
type logFilterJitterFunc func(time.Duration) time.Duration

type logFilterer interface {
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)
}

func filterLogsInChunks(ctx context.Context, client logFilterer, query ethereum.FilterQuery, from, to int64) ([]types.Log, error) {
	return filterLogsInChunksWithTiming(
		ctx,
		client,
		query,
		from,
		to,
		logFilterRequestInterval,
		waitForLogFilterDelay,
		randomLogFilterRetryJitter,
	)
}

func filterLogsInChunksWithTiming(
	ctx context.Context,
	client logFilterer,
	query ethereum.FilterQuery,
	from, to int64,
	requestInterval time.Duration,
	wait logFilterWaitFunc,
	jitter logFilterJitterFunc,
) ([]types.Log, error) {
	if from < 0 || to < 0 {
		return nil, fmt.Errorf("block range must be non-negative: %d-%d", from, to)
	}
	if from > to {
		return []types.Log{}, nil
	}

	pacedClient := &pacedLogFilterer{
		client:   client,
		interval: requestInterval,
		wait:     wait,
	}

	logs := make([]types.Log, 0)
	for chunkStart := from; chunkStart <= to; {
		chunkEnd := to
		if to-chunkStart >= maxLogFilterBlockRange {
			chunkEnd = chunkStart + maxLogFilterBlockRange - 1
		}

		chunkLogs, err := filterLogsChunk(ctx, pacedClient, query, chunkStart, chunkEnd, wait, jitter)
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

type pacedLogFilterer struct {
	client       logFilterer
	interval     time.Duration
	wait         logFilterWaitFunc
	requestCount int
}

func (p *pacedLogFilterer) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	if p.requestCount > 0 && p.interval > 0 {
		if err := p.wait(ctx, p.interval); err != nil {
			return nil, err
		}
	}
	p.requestCount++
	return p.client.FilterLogs(ctx, query)
}

func filterLogsChunk(
	ctx context.Context,
	client logFilterer,
	query ethereum.FilterQuery,
	from, to int64,
	wait logFilterWaitFunc,
	jitter logFilterJitterFunc,
) ([]types.Log, error) {
	chunkQuery := query
	chunkQuery.FromBlock = big.NewInt(from)
	chunkQuery.ToBlock = big.NewInt(to)

	logs, err := filterLogsWithRetry(ctx, client, chunkQuery, wait, jitter)
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
	leftLogs, err := filterLogsChunk(ctx, client, query, from, middle, wait, jitter)
	if err != nil {
		return nil, err
	}
	rightLogs, err := filterLogsChunk(ctx, client, query, middle+1, to, wait, jitter)
	if err != nil {
		return nil, err
	}

	return append(leftLogs, rightLogs...), nil
}

func filterLogsWithRetry(
	ctx context.Context,
	client logFilterer,
	query ethereum.FilterQuery,
	wait logFilterWaitFunc,
	jitter logFilterJitterFunc,
) ([]types.Log, error) {
	var err error
	maxAttempts := logFilterMaxAttempts
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var logs []types.Log
		logs, err = client.FilterLogs(ctx, query)
		if err == nil {
			return logs, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		rateLimited := isLogFilterRateLimitError(err)
		if rateLimited {
			maxAttempts = logFilterRateLimitMaxAttempts
		}
		if attempt == maxAttempts {
			break
		}

		delay := logFilterRetryDelay * time.Duration(1<<(attempt-1))
		if rateLimited {
			delay = logFilterRateLimitRetryDelay * time.Duration(1<<(attempt-1))
			delay += jitter(delay)
		}
		if err := wait(ctx, delay); err != nil {
			return nil, err
		}
	}

	return nil, err
}

func isLogFilterRateLimitError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "429") || strings.Contains(message, "too many requests")
}

func randomLogFilterRetryJitter(delay time.Duration) time.Duration {
	maxJitter := delay / 4
	if maxJitter <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(maxJitter) + 1))
}

func waitForLogFilterDelay(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
