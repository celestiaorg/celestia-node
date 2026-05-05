package state

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// queryRouter is a thin grpc.ClientConnInterface implementation that
// fans unary / stream invocations across multiple backend connections,
// retrying transport-class failures on the next backend in priority
// order.
//
// It is intentionally narrower than the core.MultiBlockFetcher in
// nodebuilder/core: state queries don't have a block-height notion of
// "coverage" we can gate on (most are latest-state queries against the
// app store), so the router uses pure priority-then-error fallback.
//
// Use it by wrapping a CoreAccessor's coreConns and passing the router
// to Cosmos SDK QueryClient constructors, which accept any
// grpc.ClientConnInterface. No call-site changes are needed.
type queryRouter struct {
	backends []grpc.ClientConnInterface
}

// newQueryRouter wraps a primary-first slice of *grpc.ClientConn into a
// router. Empty input yields a router that returns ErrNoBackends from
// every call — that's a programming error in callers, not a runtime
// expectation.
func newQueryRouter(conns []*grpc.ClientConn) *queryRouter {
	backends := make([]grpc.ClientConnInterface, 0, len(conns))
	for _, c := range conns {
		if c != nil {
			backends = append(backends, c)
		}
	}
	return &queryRouter{backends: backends}
}

// errNoBackends is returned when the router has no usable backends.
var errNoBackends = errors.New("state/queryRouter: no backends configured")

// Invoke implements grpc.ClientConnInterface. Tries backends in order;
// on transport-class failure (Unavailable, DeadlineExceeded), or any
// non-gRPC error, it falls back to the next backend. Application-level
// errors (e.g. NotFound, InvalidArgument) are deterministic across
// nodes for state queries and are returned immediately — falling back
// would only mask the legitimate response.
func (r *queryRouter) Invoke(
	ctx context.Context,
	method string,
	args, reply interface{},
	opts ...grpc.CallOption,
) error {
	if len(r.backends) == 0 {
		return errNoBackends
	}
	var lastErr error
	for _, b := range r.backends {
		err := b.Invoke(ctx, method, args, reply, opts...)
		if err == nil {
			return nil
		}
		if !isRetryableTransport(err) {
			return err
		}
		lastErr = err
	}
	return lastErr
}

// NewStream implements grpc.ClientConnInterface. Falls back across
// backends only at stream-establishment time; once a stream is open,
// failures mid-stream surface to the caller as usual.
func (r *queryRouter) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if len(r.backends) == 0 {
		return nil, errNoBackends
	}
	var lastErr error
	for _, b := range r.backends {
		s, err := b.NewStream(ctx, desc, method, opts...)
		if err == nil {
			return s, nil
		}
		if !isRetryableTransport(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// isRetryableTransport reports whether err warrants trying the next
// backend. We consider non-gRPC errors retryable (could be a connection
// dial issue) and the codes commonly raised by transport failures.
// Application-level codes are intentionally NOT retried: they're
// deterministic and falling back would mask them.
func isRetryableTransport(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	if !ok {
		return true
	}
	switch s.Code() {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// compile-time assertion: *queryRouter implements grpc.ClientConnInterface.
var _ grpc.ClientConnInterface = (*queryRouter)(nil)
