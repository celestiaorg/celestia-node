package shrex

import (
	"errors"

	"github.com/libp2p/go-libp2p/core/network"
)

// isResourceExhausted reports whether err represents a stream reset with the
// StreamResourceLimitExceeded code, meaning the remote peer rejected the request
// because it hit a resource limit (stream count or memory budget).
func isResourceExhausted(err error) bool {
	var streamErr *network.StreamError
	return errors.As(err, &streamErr) && streamErr.ErrorCode == network.StreamResourceLimitExceeded
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
