package shrex

import (
	"errors"

	"github.com/libp2p/go-libp2p/core/network"
)

// isResourceExhausted reports whether err represents a stream reset indicating
// the remote peer is temporarily overloaded. Two reset codes qualify:
//   - StreamResourceLimitExceeded: rcmgr rejected the stream (concurrency or memory limit)
//   - StreamRateLimited: the per-IP rate limiter rejected the stream
func isResourceExhausted(err error) bool {
	var streamErr *network.StreamError
	if !errors.As(err, &streamErr) {
		return false
	}
	return streamErr.ErrorCode == network.StreamResourceLimitExceeded ||
		streamErr.ErrorCode == network.StreamRateLimited
}

// ErrorContains reports whether any error in err's tree matches any error in targets tree.
func ErrorContains(err, target error) bool {
	if errors.Is(err, target) || target == nil {
		return true
	}

	target = errors.Unwrap(target)
	if target == nil {
		return false
	}
	return ErrorContains(err, target)
}
