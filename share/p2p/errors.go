package p2p

import (
	"context"
	"errors"
	"net"
	"time"
)

// ErrUnavailable is returned when a peer doesn't have the requested data or doesn't have the
// capacity to serve it at the moment. It is used to signal that the peer couldn't serve the data
// successfully, but should be retried later.
var ErrUnavailable = errors.New("server cannot serve the requested data at this time")

// ErrInvalidResponse is returned when a peer returns an invalid response or caused an internal
// error. It is used to signal that the peer couldn't serve the data successfully, and should not be
// retried.
var ErrInvalidResponse = errors.New("server returned an invalid response or caused an internal error")

// ExtractContextError returns the error if an underlying context error exists in the passed context
// or net.Error. This is needed because some net.Errors also mean the context deadline was exceeded,
// but yamux/mocknet do not unwrap to a context err.
func ExtractContextError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ctx.Err()
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		if deadline, _ := ctx.Deadline(); deadline.Before(time.Now()) {
			return context.DeadlineExceeded
		}
	}
	return nil
}
