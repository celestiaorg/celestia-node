package state

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeBackend implements grpc.ClientConnInterface for router tests.
type fakeBackend struct {
	name string

	invokeFn    func(method string) error
	newStreamFn func(method string) (grpc.ClientStream, error)

	invokeHits    atomic.Int64
	newStreamHits atomic.Int64
}

func (f *fakeBackend) Invoke(_ context.Context, method string, _, _ interface{}, _ ...grpc.CallOption) error {
	f.invokeHits.Add(1)
	if f.invokeFn == nil {
		return nil
	}
	return f.invokeFn(method)
}

func (f *fakeBackend) NewStream(_ context.Context, _ *grpc.StreamDesc, method string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
	f.newStreamHits.Add(1)
	if f.newStreamFn == nil {
		return nil, nil
	}
	return f.newStreamFn(method)
}

// routerWith builds a queryRouter from fake backends.
func routerWith(t *testing.T, backends ...*fakeBackend) *queryRouter {
	t.Helper()
	out := make([]grpc.ClientConnInterface, len(backends))
	for i, b := range backends {
		out[i] = b
	}
	return &queryRouter{backends: out}
}

func TestQueryRouter_PrimarySuccessShortCircuits(t *testing.T) {
	primary, secondary := &fakeBackend{name: "primary"}, &fakeBackend{name: "secondary"}
	primary.invokeFn = func(_ string) error { return nil }
	secondary.invokeFn = func(_ string) error {
		t.Fatalf("secondary must not be invoked when primary succeeds")
		return nil
	}

	r := routerWith(t, primary, secondary)
	require.NoError(t, r.Invoke(context.Background(), "/m", nil, nil))
	assert.Equal(t, int64(1), primary.invokeHits.Load())
	assert.Equal(t, int64(0), secondary.invokeHits.Load())
}

func TestQueryRouter_FallsBackOnTransportError(t *testing.T) {
	primary, secondary := &fakeBackend{}, &fakeBackend{}
	primary.invokeFn = func(_ string) error {
		return status.Error(codes.Unavailable, "primary down")
	}
	secondary.invokeFn = func(_ string) error { return nil }

	r := routerWith(t, primary, secondary)
	require.NoError(t, r.Invoke(context.Background(), "/m", nil, nil))
	assert.Equal(t, int64(1), primary.invokeHits.Load())
	assert.Equal(t, int64(1), secondary.invokeHits.Load())
}

func TestQueryRouter_DoesNotFallBackOnApplicationError(t *testing.T) {
	primary, secondary := &fakeBackend{}, &fakeBackend{}
	primary.invokeFn = func(_ string) error {
		return status.Error(codes.NotFound, "no such account")
	}
	secondary.invokeFn = func(_ string) error {
		t.Fatalf("application errors are deterministic across nodes — secondary must not be tried")
		return nil
	}

	r := routerWith(t, primary, secondary)
	err := r.Invoke(context.Background(), "/m", nil, nil)
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Equal(t, int64(1), primary.invokeHits.Load())
	assert.Equal(t, int64(0), secondary.invokeHits.Load())
}

func TestQueryRouter_AllBackendsFailReturnsLastError(t *testing.T) {
	primary, secondary := &fakeBackend{}, &fakeBackend{}
	primary.invokeFn = func(_ string) error { return status.Error(codes.Unavailable, "primary") }
	secondary.invokeFn = func(_ string) error { return status.Error(codes.DeadlineExceeded, "secondary") }

	r := routerWith(t, primary, secondary)
	err := r.Invoke(context.Background(), "/m", nil, nil)
	require.Error(t, err)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
}

func TestQueryRouter_NonGRPCErrorIsRetryable(t *testing.T) {
	primary, secondary := &fakeBackend{}, &fakeBackend{}
	primary.invokeFn = func(_ string) error { return errors.New("plain dial error") }
	secondary.invokeFn = func(_ string) error { return nil }

	r := routerWith(t, primary, secondary)
	require.NoError(t, r.Invoke(context.Background(), "/m", nil, nil))
	assert.Equal(t, int64(1), primary.invokeHits.Load())
	assert.Equal(t, int64(1), secondary.invokeHits.Load())
}

func TestQueryRouter_EmptyBackendsErrors(t *testing.T) {
	r := &queryRouter{}
	err := r.Invoke(context.Background(), "/m", nil, nil)
	require.ErrorIs(t, err, errNoBackends)

	_, err = r.NewStream(context.Background(), nil, "/m")
	require.ErrorIs(t, err, errNoBackends)
}

func TestQueryRouter_NilConnsAreSkipped(t *testing.T) {
	// newQueryRouter must not panic on nil entries — those would otherwise
	// trip the first call. They're treated as absent.
	r := newQueryRouter([]*grpc.ClientConn{nil, nil})
	err := r.Invoke(context.Background(), "/m", nil, nil)
	require.ErrorIs(t, err, errNoBackends)
}

func TestQueryRouter_StreamFallback(t *testing.T) {
	primary, secondary := &fakeBackend{}, &fakeBackend{}
	primary.newStreamFn = func(_ string) (grpc.ClientStream, error) {
		return nil, status.Error(codes.Unavailable, "primary")
	}
	secondary.newStreamFn = func(_ string) (grpc.ClientStream, error) {
		return nil, nil // success with nil stream is fine for the test
	}

	r := routerWith(t, primary, secondary)
	_, err := r.NewStream(context.Background(), nil, "/m")
	require.NoError(t, err)
	assert.Equal(t, int64(1), primary.newStreamHits.Load())
	assert.Equal(t, int64(1), secondary.newStreamHits.Load())
}

func TestIsRetryableTransport(t *testing.T) {
	assert.False(t, isRetryableTransport(nil))
	assert.True(t, isRetryableTransport(errors.New("not a status")))
	assert.True(t, isRetryableTransport(status.Error(codes.Unavailable, "")))
	assert.True(t, isRetryableTransport(status.Error(codes.DeadlineExceeded, "")))
	assert.False(t, isRetryableTransport(status.Error(codes.NotFound, "")))
	assert.False(t, isRetryableTransport(status.Error(codes.InvalidArgument, "")))
	assert.False(t, isRetryableTransport(status.Error(codes.PermissionDenied, "")))
}
