package headers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/header"
)

const (
	maxRangeRequest       = 64
	headerFetchRetryDelay = 2 * time.Second
)

type exchanger interface {
	GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error)
	GetRangeByHeight(
		ctx context.Context,
		from *header.ExtendedHeader,
		to uint64,
	) ([]*header.ExtendedHeader, error)
}

func fetchChunkWithRetry(
	ctx context.Context,
	ex exchanger,
	c chunk,
	requestTimeout time.Duration,
) ([]*header.ExtendedHeader, error) {
	return retryFetch(ctx, requestTimeout,
		func(ctx context.Context) ([]*header.ExtendedHeader, error) {
			hdrs, err := fetchChunkUsingRemoteAnchor(ctx, ex, c)
			if err != nil {
				return nil, fmt.Errorf("fetch chunk %d range %d..%d: %w",
					c.index, c.start, c.end, err)
			}
			return hdrs, nil
		},
		func(attempt uint64, err error) {
			log.Warnw("fetch chunk failed; retrying",
				"chunk", c.index, "start", c.start, "end", c.end,
				"attempt", attempt, "error", err)
		},
	)
}

// fetchChunkUsingRemoteAnchor pulls a single chunk via two RPCs: a
// GetByHeight to anchor on (start-1), then a GetRangeByHeight up to (end+1).
// The anchor is the source peer's, not the local trusted tip; the writer
// re-verifies the returned chunk against the local tip before append.
func fetchChunkUsingRemoteAnchor(
	ctx context.Context,
	ex exchanger,
	c chunk,
) ([]*header.ExtendedHeader, error) {
	anchor, err := ex.GetByHeight(ctx, c.start-1)
	if err != nil {
		return nil, fmt.Errorf("fetch peer anchor %d: %w", c.start-1, err)
	}
	hdrs, err := ex.GetRangeByHeight(ctx, anchor, c.end+1)
	if err != nil {
		return nil, fmt.Errorf("fetch range %d..%d: %w", c.start, c.end, err)
	}
	return hdrs, nil
}

// getByHeightWithRetry fetches a single header with constant-delay retry. Used
// for the genesis anchor when starting from height 1.
func getByHeightWithRetry(
	ctx context.Context,
	ex exchanger,
	height uint64,
	requestTimeout time.Duration,
) (*header.ExtendedHeader, error) {
	return retryFetch(ctx, requestTimeout,
		func(ctx context.Context) (*header.ExtendedHeader, error) {
			h, err := ex.GetByHeight(ctx, height)
			if err != nil {
				return nil, fmt.Errorf("get-by-height %d: %w", height, err)
			}
			return h, nil
		},
		func(attempt uint64, err error) {
			log.Warnw("get-by-height failed; retrying",
				"height", height, "attempt", attempt, "error", err)
		},
	)
}

// retryFetch runs op under a per-attempt timeout, retrying with a constant
// delay until success, ctx cancel, or a non-retryable error. onRetry is
// invoked before each sleep so the caller can log attempt details.
func retryFetch[T any](
	ctx context.Context,
	requestTimeout time.Duration,
	op func(ctx context.Context) (T, error),
	onRetry func(attempt uint64, err error),
) (T, error) {
	var zero T
	for attempt := uint64(1); ; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		result, err := op(reqCtx)
		cancel()
		if err == nil {
			return result, nil
		}
		if !isRetryableHeaderFetchErr(err) {
			return zero, err
		}
		onRetry(attempt, err)
		select {
		case <-time.After(headerFetchRetryDelay):
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}
}

func isRetryableHeaderFetchErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	return true
}
