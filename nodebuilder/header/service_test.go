package header

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
)

func TestGetByHeightHandlesError(t *testing.T) {
	serv := Service{
		syncer: &errorSyncer[*header.ExtendedHeader]{},
	}

	assert.NotPanics(t, func() {
		h, err := serv.GetByHeight(context.Background(), 100)
		assert.Error(t, err)
		assert.Nil(t, h)
	})
}

type errorSyncer[H libhead.Header[H]] struct{}

func (d *errorSyncer[H]) Head(context.Context, ...libhead.HeadOption[H]) (H, error) {
	var zero H
	return zero, fmt.Errorf("dummy error")
}

func (d *errorSyncer[H]) State() sync.State {
	return sync.State{}
}

func (d *errorSyncer[H]) SyncWait(context.Context) error {
	return fmt.Errorf("dummy error")
}

// TestGetByHeightBelowTailButAvailable tests the fix for issue #4603
// where blocks above tail but within prune window should be accessible
func TestGetByHeightBelowTailButAvailable(t *testing.T) {
	// Create a mock store that has a header available even though it's below the tail
	mockStore := &mockStore{
		tailHeight: 100,
		availableHeaders: map[uint64]*header.ExtendedHeader{
			95: createMockHeader(95),
		},
	}

	serv := Service{
		syncer: &mockSyncer[*header.ExtendedHeader]{
			head: createMockHeader(110),
		},
		store: mockStore,
	}

	// Test that we can get a header that's below tail but still available
	h, err := serv.GetByHeight(context.Background(), 95)
	require.NoError(t, err)
	require.NotNil(t, h)
	assert.Equal(t, uint64(95), h.Height())
}

// TestGetByHeightBelowTailNotAvailable tests that we still get an error
// when a header is truly below tail and not available
func TestGetByHeightBelowTailNotAvailable(t *testing.T) {
	// Create a mock store that doesn't have the requested header
	mockStore := &mockStore{
		tailHeight:       100,
		availableHeaders: map[uint64]*header.ExtendedHeader{
			// No header at height 90
		},
	}

	serv := Service{
		syncer: &mockSyncer[*header.ExtendedHeader]{
			head: createMockHeader(110),
		},
		store: mockStore,
	}

	// Test that we get an error for a header that's truly below tail
	h, err := serv.GetByHeight(context.Background(), 90)
	assert.Error(t, err)
	assert.Nil(t, h)
	assert.Contains(t, err.Error(), "requested header (90) is below Tail (100)")
}

// Mock implementations for testing
type mockStore struct {
	tailHeight       uint64
	availableHeaders map[uint64]*header.ExtendedHeader
}

func (m *mockStore) Tail(ctx context.Context) (*header.ExtendedHeader, error) {
	return createMockHeader(m.tailHeight), nil
}

func (m *mockStore) Head(ctx context.Context, opts ...libhead.HeadOption[*header.ExtendedHeader]) (*header.ExtendedHeader, error) {
	// Return the highest available header
	var maxHeight uint64
	for height := range m.availableHeaders {
		if height > maxHeight {
			maxHeight = height
		}
	}
	return createMockHeader(maxHeight), nil
}

func (m *mockStore) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	if header, exists := m.availableHeaders[height]; exists {
		return header, nil
	}
	return nil, fmt.Errorf("header not found at height %d", height)
}

func (m *mockStore) Get(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockStore) GetRangeByHeight(ctx context.Context, from *header.ExtendedHeader, to uint64) ([]*header.ExtendedHeader, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockStore) Append(ctx context.Context, headers ...*header.ExtendedHeader) error {
	// Mock implementation - just store the headers
	for _, h := range headers {
		m.availableHeaders[h.Height()] = h
	}
	return nil
}

func (m *mockStore) DeleteTo(ctx context.Context, height uint64) error {
	// Mock implementation - remove headers up to the given height
	for h := range m.availableHeaders {
		if h <= height {
			delete(m.availableHeaders, h)
		}
	}
	return nil
}

func (m *mockStore) GetRange(ctx context.Context, from uint64, amount uint64) ([]*header.ExtendedHeader, error) {
	// Mock implementation - return headers in range
	var result []*header.ExtendedHeader
	for i := uint64(0); i < amount; i++ {
		height := from + i
		if header, exists := m.availableHeaders[height]; exists {
			result = append(result, header)
		}
	}
	return result, nil
}

func (m *mockStore) Has(ctx context.Context, hash libhead.Hash) (bool, error) {
	// Mock implementation - check if header exists by hash
	for _, header := range m.availableHeaders {
		if bytes.Equal(header.Hash(), hash) {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockStore) HasAt(ctx context.Context, height uint64) bool {
	// Mock implementation - check if header exists at height
	_, exists := m.availableHeaders[height]
	return exists
}

func (m *mockStore) Height() uint64 {
	// Mock implementation - return the highest available height
	var maxHeight uint64
	for height := range m.availableHeaders {
		if height > maxHeight {
			maxHeight = height
		}
	}
	return maxHeight
}

func (m *mockStore) OnDelete(fn func(context.Context, uint64) error) {
	// Mock implementation - no-op for testing
}

type mockSyncer[H libhead.Header[H]] struct {
	head H
}

func (m *mockSyncer[H]) Head(context.Context, ...libhead.HeadOption[H]) (H, error) {
	return m.head, nil
}

func (m *mockSyncer[H]) State() sync.State {
	return sync.State{}
}

func (m *mockSyncer[H]) SyncWait(context.Context) error {
	return nil
}

func createMockHeader(height uint64) *header.ExtendedHeader {
	// Create a minimal mock header for testing
	// We need to set the RawHeader.Height field for the Height() method to work
	return &header.ExtendedHeader{
		RawHeader: header.RawHeader{
			Height: int64(height),
		},
	}
}
