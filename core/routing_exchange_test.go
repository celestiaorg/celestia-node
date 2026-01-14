package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

func TestRoutingExchange_GetByHeight_RoutesToCore(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	// Create test headers
	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}
	coreEx.head = headers[9] // head at height 10

	// Configure window to produce cutoff = 5
	// With head=10 we want 10 - (window/blockTime) = 5
	// So blocksInWindow = 5 meaning window/blockTime = 5
	blockTime := time.Second
	window := 5 * blockTime

	routingEx := NewRoutingExchange(coreEx, p2pEx,
		WithStorageWindow(window),
		WithBlockTime(blockTime),
	)

	ctx := context.Background()

	// Call Head first to initialize the cutoff
	_, err := routingEx.Head(ctx)
	require.NoError(t, err)

	coreEx.calls = 0 // Reset call count after Head

	// Height 7 > cutoff 5 should use core
	_, err = routingEx.GetByHeight(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_GetByHeight_RoutesToP2P(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}
	coreEx.head = headers[9] // head at height 10

	blockTime := time.Second
	window := 5 * blockTime // cutoff = 10 - 5 = 5

	routingEx := NewRoutingExchange(coreEx, p2pEx,
		WithStorageWindow(window),
		WithBlockTime(blockTime),
	)

	ctx := context.Background()

	// Call Head first to initialize the cutoff
	_, err := routingEx.Head(ctx)
	require.NoError(t, err)

	coreEx.calls = 0
	p2pEx.calls = 0

	// Height 3 <= cutoff 5, should use P2P
	_, err = routingEx.GetByHeight(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, 0, coreEx.calls)
	assert.Equal(t, 1, p2pEx.calls)
}

func TestRoutingExchange_GetByHeight_NoCutoffBeforeHead(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}

	routingEx := NewRoutingExchange(coreEx, p2pEx,
		WithStorageWindow(time.Hour),
		WithBlockTime(time.Second),
	)

	ctx := context.Background()

	// Without calling Head first cutoff is 0, so all heights > 0 go to core
	_, err := routingEx.GetByHeight(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_GetByHeight_NoWindow(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}
	coreEx.head = headers[9]

	routingEx := NewRoutingExchange(coreEx, p2pEx)
	// No window/blockTime set - cutoff will remain 0

	ctx := context.Background()

	// Even after Head cutoff stays 0 because window is not configured
	_, err := routingEx.Head(ctx)
	require.NoError(t, err)

	coreEx.calls = 0

	// With cutoff=0 and height 3 > 0 we should use core
	_, err = routingEx.GetByHeight(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_GetRangeByHeight_AllInWindow(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}
	coreEx.head = headers[9] // head at height 10

	blockTime := time.Second
	window := 8 * blockTime // cutoff = 10 - 8 = 2

	routingEx := NewRoutingExchange(coreEx, p2pEx,
		WithStorageWindow(window),
		WithBlockTime(blockTime),
	)

	ctx := context.Background()

	// Call Head first to initialize the cutoff
	_, err := routingEx.Head(ctx)
	require.NoError(t, err)

	coreEx.calls = 0

	// Request range 5-8, cutoff is 2 and all heights > 2 so all in window
	_, err = routingEx.GetRangeByHeight(ctx, headers[4], 8)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_GetRangeByHeight_AllOutsideWindow(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}
	coreEx.head = headers[9] // head at height 10

	blockTime := time.Second
	window := 2 * blockTime // cutoff = 10 - 2 = 8

	routingEx := NewRoutingExchange(coreEx, p2pEx,
		WithStorageWindow(window),
		WithBlockTime(blockTime),
	)

	ctx := context.Background()

	// Call Head first to initialize the cutoff
	_, err := routingEx.Head(ctx)
	require.NoError(t, err)

	coreEx.calls = 0

	// Request range 2-5, cutoff is 8, all heights <= 8 so all outside window
	_, err = routingEx.GetRangeByHeight(ctx, headers[1], 5)
	require.NoError(t, err)
	assert.Equal(t, 0, coreEx.calls)
	assert.Equal(t, 1, p2pEx.calls)
}

func TestRoutingExchange_GetRangeByHeight_Split(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}
	coreEx.head = headers[9] // head at height 10

	blockTime := time.Second
	window := 5 * blockTime // cutoff = 10 - 5 = 5

	routingEx := NewRoutingExchange(coreEx, p2pEx,
		WithStorageWindow(window),
		WithBlockTime(blockTime),
	)

	ctx := context.Background()

	// Call Head first to initialize the cutoff
	_, err := routingEx.Head(ctx)
	require.NoError(t, err)

	coreEx.calls = 0

	// Request range 3-8, cutoff is 5
	// Heights 3-5 should come from P2P (outside window)
	// Heights 6-8 should come from core (inside window)
	result, err := routingEx.GetRangeByHeight(ctx, headers[2], 8)
	require.NoError(t, err)
	assert.Len(t, result, 5) // heights 4,5,6,7,8
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 1, p2pEx.calls)
}

func TestRoutingExchange_Head_AlwaysUsesCore(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(5)
	coreEx.head = headers[4]
	p2pEx.head = headers[4]

	blockTime := time.Second
	window := 100 * blockTime // high cutoff

	routingEx := NewRoutingExchange(coreEx, p2pEx,
		WithStorageWindow(window),
		WithBlockTime(blockTime),
	)

	ctx := context.Background()

	// Head should always use core regardless of cutoff
	_, err := routingEx.Head(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_Head_UpdatesCutoff(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(20)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}

	blockTime := time.Second
	window := 5 * blockTime

	routingEx := NewRoutingExchange(coreEx, p2pEx,
		WithStorageWindow(window),
		WithBlockTime(blockTime),
	)

	ctx := context.Background()

	// First head at height 10, cutoff = 10 - 5 = 5
	coreEx.head = headers[9]
	_, err := routingEx.Head(ctx)
	require.NoError(t, err)

	coreEx.calls = 0
	p2pEx.calls = 0

	// Height 7 > cutoff 5, should use core
	_, err = routingEx.GetByHeight(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)

	// Now head advances to height 20, cutoff = 20 - 5 = 15
	coreEx.head = headers[19]
	_, err = routingEx.Head(ctx)
	require.NoError(t, err)

	coreEx.calls = 0
	p2pEx.calls = 0

	// Height 7 <= new cutoff 15, should now use P2P
	_, err = routingEx.GetByHeight(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, 0, coreEx.calls)
	assert.Equal(t, 1, p2pEx.calls)
}

func TestRoutingExchange_Get_TriesCoreFirst(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(5)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}

	routingEx := NewRoutingExchange(coreEx, p2pEx)

	ctx := context.Background()

	// Get by hash tries core first
	_, err := routingEx.Get(ctx, headers[2].Hash())
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_Get_FallsBackToP2P(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuiteDefaults(t)
	headers := suite.GenExtendedHeaders(5)
	// Only add to p2p, not core
	for _, h := range headers {
		p2pEx.addHeader(h)
	}

	routingEx := NewRoutingExchange(coreEx, p2pEx)

	ctx := context.Background()

	// Get by hash should fall back to p2p if core fails
	_, err := routingEx.Get(ctx, headers[2].Hash())
	require.NoError(t, err)
	assert.Equal(t, 0, coreEx.calls) // core tried but not counted (returned error)
	assert.Equal(t, 1, p2pEx.calls)
}

// mockExchange is a mock implementation of libhead.Exchange for testing
type mockExchange struct {
	headers map[uint64]*header.ExtendedHeader
	head    *header.ExtendedHeader
	headErr error
	calls   int
}

func newMockExchange() *mockExchange {
	return &mockExchange{
		headers: make(map[uint64]*header.ExtendedHeader),
	}
}

func (m *mockExchange) Head(
	_ context.Context,
	_ ...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	m.calls++
	if m.headErr != nil {
		return nil, m.headErr
	}
	if m.head == nil {
		return nil, errors.New("no head")
	}
	return m.head, nil
}

func (m *mockExchange) Get(_ context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	for _, h := range m.headers {
		if string(h.Hash()) == string(hash) {
			m.calls++
			return h, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockExchange) GetByHeight(_ context.Context, height uint64) (*header.ExtendedHeader, error) {
	h, ok := m.headers[height]
	if !ok {
		return nil, errors.New("not found")
	}
	m.calls++
	return h, nil
}

func (m *mockExchange) GetRangeByHeight(
	_ context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	var result []*header.ExtendedHeader
	for i := from.Height() + 1; i <= to; i++ {
		h, ok := m.headers[i]
		if !ok {
			return nil, errors.New("not found")
		}
		result = append(result, h)
	}
	m.calls++
	return result, nil
}

func (m *mockExchange) addHeader(h *header.ExtendedHeader) {
	m.headers[h.Height()] = h
}
