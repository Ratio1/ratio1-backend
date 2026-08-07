package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

type logFiltererMock struct {
	queries     []ethereum.FilterQuery
	failAt      int
	failAlways  bool
	failure     error
	resultCount func(query ethereum.FilterQuery) int
}

func (m *logFiltererMock) FilterLogs(_ context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	m.queries = append(m.queries, query)
	if m.failAt > 0 && (len(m.queries) == m.failAt || m.failAlways && len(m.queries) >= m.failAt) {
		if m.failure != nil {
			return nil, m.failure
		}
		return nil, errors.New("provider error")
	}

	count := 1
	if m.resultCount != nil {
		count = m.resultCount(query)
	}
	logs := make([]types.Log, count)
	for i := range logs {
		logs[i].BlockNumber = uint64(query.FromBlock.Int64())
	}
	return logs, nil
}

func filterLogsInChunksForTest(
	ctx context.Context,
	client logFilterer,
	query ethereum.FilterQuery,
	from, to int64,
) ([]types.Log, error) {
	return filterLogsInChunksWithTiming(ctx, client, query, from, to, 0, noLogFilterWait, noLogFilterJitter)
}

func noLogFilterWait(ctx context.Context, _ time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func noLogFilterJitter(time.Duration) time.Duration {
	return 0
}

func TestFilterLogsInChunks_ShouldSplitInclusiveBlockRange(t *testing.T) {
	client := &logFiltererMock{}
	address := common.HexToAddress("0x0000000000000000000000000000000000000001")
	topic := common.HexToHash("0x01")
	query := ethereum.FilterQuery{
		Addresses: []common.Address{address},
		Topics:    [][]common.Hash{{topic}},
	}

	logs, err := filterLogsInChunksForTest(context.Background(), client, query, 100, 20_100)

	require.NoError(t, err)
	require.Len(t, client.queries, 3)
	require.Equal(t, int64(100), client.queries[0].FromBlock.Int64())
	require.Equal(t, int64(10_099), client.queries[0].ToBlock.Int64())
	require.Equal(t, int64(10_100), client.queries[1].FromBlock.Int64())
	require.Equal(t, int64(20_099), client.queries[1].ToBlock.Int64())
	require.Equal(t, int64(20_100), client.queries[2].FromBlock.Int64())
	require.Equal(t, int64(20_100), client.queries[2].ToBlock.Int64())
	require.Equal(t, query.Addresses, client.queries[0].Addresses)
	require.Equal(t, query.Topics, client.queries[0].Topics)
	require.Equal(t, []uint64{100, 10_100, 20_100}, []uint64{
		logs[0].BlockNumber,
		logs[1].BlockNumber,
		logs[2].BlockNumber,
	})
	require.Nil(t, query.FromBlock)
	require.Nil(t, query.ToBlock)
}

func TestFilterLogsInChunks_ShouldUseOneRequestAtRangeLimit(t *testing.T) {
	client := &logFiltererMock{}

	_, err := filterLogsInChunksForTest(context.Background(), client, ethereum.FilterQuery{}, 50, 10_049)

	require.NoError(t, err)
	require.Len(t, client.queries, 1)
	require.Equal(t, int64(50), client.queries[0].FromBlock.Int64())
	require.Equal(t, int64(10_049), client.queries[0].ToBlock.Int64())
}

func TestFilterLogsInChunks_ShouldReturnEmptyForReversedRange(t *testing.T) {
	client := &logFiltererMock{}

	logs, err := filterLogsInChunksForTest(context.Background(), client, ethereum.FilterQuery{}, 11, 10)

	require.NoError(t, err)
	require.Empty(t, logs)
	require.Empty(t, client.queries)
}

func TestFilterLogsInChunks_ShouldStopAtFirstError(t *testing.T) {
	client := &logFiltererMock{failAt: 2, failAlways: true}

	logs, err := filterLogsInChunksForTest(context.Background(), client, ethereum.FilterQuery{}, 1, 20_000)

	require.Nil(t, logs)
	require.EqualError(t, err, "filtering logs for blocks 10001-20000: provider error")
	require.Len(t, client.queries, 4)
}

func TestFilterLogsInChunks_ShouldRetryTransientError(t *testing.T) {
	client := &logFiltererMock{failAt: 1}

	logs, err := filterLogsInChunksForTest(context.Background(), client, ethereum.FilterQuery{}, 1, 1)

	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Len(t, client.queries, 2)
}

func TestFilterLogsInChunks_ShouldStopRetryingWhenContextIsCanceled(t *testing.T) {
	client := &logFiltererMock{failAt: 1, failAlways: true}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logs, err := filterLogsInChunksForTest(ctx, client, ethereum.FilterQuery{}, 1, 1)

	require.Nil(t, logs)
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, client.queries, 1)
}

func TestFilterLogsInChunks_ShouldSplitResultCappedRange(t *testing.T) {
	client := &logFiltererMock{
		resultCount: func(query ethereum.FilterQuery) int {
			if query.FromBlock.Int64() == 1 && query.ToBlock.Int64() == 10_000 {
				return maxLogFilterResults
			}
			return maxLogFilterResults / 2
		},
	}

	logs, err := filterLogsInChunksForTest(context.Background(), client, ethereum.FilterQuery{}, 1, 10_000)

	require.NoError(t, err)
	require.Len(t, logs, maxLogFilterResults)
	require.Len(t, client.queries, 3)
	require.Equal(t, int64(1), client.queries[1].FromBlock.Int64())
	require.Equal(t, int64(5_000), client.queries[1].ToBlock.Int64())
	require.Equal(t, int64(5_001), client.queries[2].FromBlock.Int64())
	require.Equal(t, int64(10_000), client.queries[2].ToBlock.Int64())
}

func TestFilterLogsInChunks_ShouldRejectCappedSingleBlock(t *testing.T) {
	client := &logFiltererMock{
		resultCount: func(ethereum.FilterQuery) int {
			return maxLogFilterResults
		},
	}

	logs, err := filterLogsInChunksForTest(context.Background(), client, ethereum.FilterQuery{}, 42, 42)

	require.Nil(t, logs)
	require.EqualError(t, err, "filtering logs for block 42 returned 10000 results; completeness cannot be guaranteed")
	require.Len(t, client.queries, 1)
}

func TestFilterLogsInChunks_ShouldRejectNegativeBlock(t *testing.T) {
	client := &logFiltererMock{}

	logs, err := filterLogsInChunksForTest(context.Background(), client, ethereum.FilterQuery{}, -1, 10)

	require.Nil(t, logs)
	require.EqualError(t, err, "block range must be non-negative: -1-10")
	require.Empty(t, client.queries)
}

func TestFilterLogsInChunks_ShouldHandleMaximumInt64Boundary(t *testing.T) {
	client := &logFiltererMock{}
	from := int64(math.MaxInt64 - maxLogFilterBlockRange)

	_, err := filterLogsInChunksForTest(context.Background(), client, ethereum.FilterQuery{}, from, math.MaxInt64)

	require.NoError(t, err)
	require.Len(t, client.queries, 2)
	require.Equal(t, from, client.queries[0].FromBlock.Int64())
	require.Equal(t, int64(math.MaxInt64-1), client.queries[0].ToBlock.Int64())
	require.Equal(t, int64(math.MaxInt64), client.queries[1].FromBlock.Int64())
	require.Equal(t, int64(math.MaxInt64), client.queries[1].ToBlock.Int64())
}

func TestFilterLogsInChunks_ShouldPaceEveryRequestAfterFirst(t *testing.T) {
	client := &logFiltererMock{}
	var delays []time.Duration
	wait := func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}

	_, err := filterLogsInChunksWithTiming(
		context.Background(),
		client,
		ethereum.FilterQuery{},
		1,
		20_001,
		time.Second,
		wait,
		noLogFilterJitter,
	)

	require.NoError(t, err)
	require.Len(t, client.queries, 3)
	require.Equal(t, []time.Duration{time.Second, time.Second}, delays)
}

func TestFilterLogsInChunks_ShouldUseLongBackoffForRateLimit(t *testing.T) {
	client := &logFiltererMock{
		failAt:     1,
		failAlways: true,
		failure:    errors.New("429 Too Many Requests"),
	}
	var delays []time.Duration
	var jitterBases []time.Duration
	wait := func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}
	jitter := func(delay time.Duration) time.Duration {
		jitterBases = append(jitterBases, delay)
		return 100 * time.Millisecond
	}

	logs, err := filterLogsInChunksWithTiming(
		context.Background(),
		client,
		ethereum.FilterQuery{},
		1,
		1,
		0,
		wait,
		jitter,
	)

	require.Nil(t, logs)
	require.EqualError(t, err, "filtering logs for blocks 1-1: 429 Too Many Requests")
	require.Len(t, client.queries, logFilterRateLimitMaxAttempts)
	require.Equal(t, []time.Duration{
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
	}, jitterBases)
	require.Equal(t, []time.Duration{
		2*time.Second + 100*time.Millisecond,
		4*time.Second + 100*time.Millisecond,
		8*time.Second + 100*time.Millisecond,
		16*time.Second + 100*time.Millisecond,
		32*time.Second + 100*time.Millisecond,
	}, delays)
}

func TestRandomLogFilterRetryJitter_ShouldStayWithinQuarterDelay(t *testing.T) {
	delay := 8 * time.Second
	for i := 0; i < 100; i++ {
		jitter := randomLogFilterRetryJitter(delay)
		require.GreaterOrEqual(t, jitter, time.Duration(0))
		require.LessOrEqual(t, jitter, delay/4)
	}
}
