package mocks

import (
	"bytes"
	"context"
	"testing"

	"github.com/celestiaorg/celestia-node/header"
	headerpkg "github.com/celestiaorg/celestia-node/pkg/header"
)

type MockStore struct {
	Headers    map[int64]*header.ExtendedHeader
	HeadHeight int64
}

// NewStore creates a mock store and adds several random
// headers
func NewStore(t *testing.T, numHeaders int) *MockStore {
	store := &MockStore{
		Headers:    make(map[int64]*header.ExtendedHeader),
		HeadHeight: 0,
	}

	suite := header.NewTestSuite(t, numHeaders)

	for i := 0; i < numHeaders; i++ {
		header := suite.GenExtendedHeader()
		store.Headers[header.Height] = header

		if header.Height > store.HeadHeight {
			store.HeadHeight = header.Height
		}
	}
	return store
}

func (m *MockStore) Init(context.Context, *header.ExtendedHeader) error { return nil }
func (m *MockStore) Start(context.Context) error                        { return nil }
func (m *MockStore) Stop(context.Context) error                         { return nil }

func (m *MockStore) Height() uint64 {
	return uint64(m.HeadHeight)
}

func (m *MockStore) Head(context.Context) (*header.ExtendedHeader, error) {
	return m.Headers[m.HeadHeight], nil
}

func (m *MockStore) Get(ctx context.Context, hash headerpkg.Hash) (*header.ExtendedHeader, error) {
	for _, header := range m.Headers {
		if bytes.Equal(header.Hash(), hash) {
			return header, nil
		}
	}
	return nil, header.ErrNotFound
}

func (m *MockStore) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return m.Headers[int64(height)], nil
}

func (m *MockStore) GetRangeByHeight(ctx context.Context, from, to uint64) ([]*header.ExtendedHeader, error) {
	headers := make([]*header.ExtendedHeader, to-from)
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

func (m *MockStore) GetVerifiedRange(
	ctx context.Context,
	h *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	return m.GetRangeByHeight(ctx, uint64(h.Height)+1, to)
}

func (m *MockStore) Has(context.Context, headerpkg.Hash) (bool, error) {
	return false, nil
}

func (m *MockStore) Append(ctx context.Context, headers ...*header.ExtendedHeader) (int, error) {
	for _, header := range headers {
		m.Headers[header.Height] = header
		// set head
		if header.Height > m.HeadHeight {
			m.HeadHeight = header.Height
		}
	}
	return len(headers), nil
}
