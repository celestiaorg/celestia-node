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

// streamServeErr is the reset code the server uses when a response fails on its own side after the
// OK status was sent, e.g. a file read error or the request timeout firing mid-stream. It sits
// outside the 0x1000 range libp2p reserves for its own codes.
const streamServeErr network.StreamErrorCode = 0x5300

// isServeErr reports whether err is a reset by the remote server with streamServeErr, meaning the
// server failed to produce the response rather than sending invalid data.
func isServeErr(err error) bool {
	var streamErr *network.StreamError
	if !errors.As(err, &streamErr) {
		return false
	}
	return streamErr.Remote && streamErr.ErrorCode == streamServeErr
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
