package mocks

import (
	"bytes"
	"context"
	"testing"

	"github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/test"
)

type MockStore[H header.Header] struct {
	Headers    map[int64]H
	HeadHeight int64
}

// NewStore creates a mock store and adds several random
// headers
func NewStore[H header.Header](t *testing.T, gen test.Generator[H], numHeaders int) *MockStore[H] {
	store := &MockStore[H]{
		Headers:    make(map[int64]H),
		HeadHeight: 0,
	}

	for i := 0; i < numHeaders; i++ {
		header := gen.GetRandomHeader()
		store.Headers[header.Height()] = header

		if header.Height() > store.HeadHeight {
			store.HeadHeight = header.Height()
		}
	}
	return store
}

func (m *MockStore[H]) Init(context.Context, H) error { return nil }
func (m *MockStore[H]) Start(context.Context) error   { return nil }
func (m *MockStore[H]) Stop(context.Context) error    { return nil }

func (m *MockStore[H]) Height() uint64 {
	return uint64(m.HeadHeight)
}

func (m *MockStore[H]) Head(context.Context) (H, error) {
	return m.Headers[m.HeadHeight], nil
}

func (m *MockStore[H]) Get(ctx context.Context, hash header.Hash) (H, error) {
	for _, header := range m.Headers {
		if bytes.Equal(header.Hash(), hash) {
			return header, nil
		}
	}
	var zero H
	return zero, header.ErrNotFound
}

func (m *MockStore[H]) GetByHeight(ctx context.Context, height uint64) (H, error) {
	return m.Headers[int64(height)], nil
}

func (m *MockStore[H]) GetRangeByHeight(ctx context.Context, from, to uint64) ([]H, error) {
	headers := make([]H, to-from)
	// As the requested range is [from; to),
	// check that (to-1) height in request is less than
	// the biggest header height in store.
	if to-1 > m.Height() {
		return nil, header.ErrNotFound
	}
	for i := range headers {
		headers[i] = m.Headers[int64(from)]
		from++
	}
	return headers, nil
}

func (m *MockStore[H]) GetVerifiedRange(
	ctx context.Context,
	h H,
	to uint64,
) ([]H, error) {
	return m.GetRangeByHeight(ctx, uint64(h.Height())+1, to)
}

func (m *MockStore[H]) Has(context.Context, header.Hash) (bool, error) {
	return false, nil
}

func (m *MockStore[H]) HasAt(_ context.Context, height uint64) bool {
	return height != 0 && m.HeadHeight >= int64(height)
}

func (m *MockStore[H]) Append(ctx context.Context, headers ...H) error {
	for _, header := range headers {
		m.Headers[header.Height()] = header
		// set head
		if header.Height() > m.HeadHeight {
			m.HeadHeight = header.Height()
		}
	}
	return nil
}
