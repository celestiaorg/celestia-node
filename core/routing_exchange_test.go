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

func TestRoutingExchange_GetByHeight_AlwaysUsesCore(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}

	// GetByHeight always routes to core regardless of window settings
	routingEx, err := NewRoutingExchange(coreEx, p2pEx,
		time.Hour,
		time.Second,
	)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = routingEx.GetByHeight(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_GetRangeByHeight_AllInWindow(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}

	blockTime := time.Second
	window := time.Hour

	routingEx, err := NewRoutingExchange(coreEx, p2pEx,
		window,
		blockTime,
	)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = routingEx.GetRangeByHeight(ctx, headers[4], 8)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_GetRangeByHeight_AllOutsideWindow(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}

	blockTime := time.Nanosecond
	window := time.Nanosecond

	routingEx, err := NewRoutingExchange(coreEx, p2pEx,
		window,
		blockTime,
	)
	require.NoError(t, err)

	_, err = routingEx.GetRangeByHeight(context.Background(), headers[1], 5)
	require.NoError(t, err)
	assert.Equal(t, 0, coreEx.calls)
	assert.Equal(t, 1, p2pEx.calls)
}

func TestRoutingExchange_GetRangeByHeight_Split(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}

	blockTime := time.Second
	window := 5 * time.Second

	fromHeader := headers[2] // height 3
	oldTime := time.Now().Add(-window - 2*blockTime)
	fromHeader.RawHeader.Time = oldTime

	routingEx, err := NewRoutingExchange(coreEx, p2pEx,
		window,
		blockTime,
	)
	require.NoError(t, err)
	ctx := context.Background()

	result, err := routingEx.GetRangeByHeight(ctx, fromHeader, 8)
	require.NoError(t, err)
	require.Len(t, result, 4) // heights 4,5,6,7 (`to` is exclusive)
	assert.Equal(t, uint64(4), result[0].Height())
	assert.Equal(t, uint64(7), result[len(result)-1].Height())
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 1, p2pEx.calls)
}

// TestRoutingExchange_GetRangeByHeight_SplitCutoffUnavailableInCore ensures the
// split routes the cutoff height itself to P2P. The core exchange serves only
// heights inside the window (the consensus node has pruned everything at and
// below the cutoff), so requesting the cutoff from core fails the whole range.
func TestRoutingExchange_GetRangeByHeight_SplitCutoffUnavailableInCore(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(10)

	blockTime := time.Second
	window := 5 * time.Second

	fromHeader := headers[2] // height 3
	oldTime := time.Now().Add(-window - 2*blockTime)
	fromHeader.RawHeader.Time = oldTime
	// cutoff resolves to height 5: heights <= 5 are pruned from core,
	// heights > 5 are inside the window.
	const cutoff = 5
	for _, h := range headers {
		if h.Height() <= cutoff {
			p2pEx.addHeader(h)
		} else {
			coreEx.addHeader(h)
		}
	}

	routingEx, err := NewRoutingExchange(coreEx, p2pEx,
		window,
		blockTime,
	)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("split range crossing the cutoff", func(t *testing.T) {
		result, err := routingEx.GetRangeByHeight(ctx, fromHeader, 8)
		require.NoError(t, err)
		require.Len(t, result, 4) // heights 4,5,6,7
		assert.Equal(t, uint64(4), result[0].Height())
		assert.Equal(t, uint64(7), result[len(result)-1].Height())
	})

	t.Run("range ending exactly at the cutoff", func(t *testing.T) {
		// to = cutoff+1 requests heights 4,5 — entirely outside the window,
		// so core must not be involved at all.
		coreCalls := coreEx.calls
		result, err := routingEx.GetRangeByHeight(ctx, fromHeader, cutoff+1)
		require.NoError(t, err)
		require.Len(t, result, 2) // heights 4,5
		assert.Equal(t, uint64(cutoff), result[len(result)-1].Height())
		assert.Equal(t, coreCalls, coreEx.calls)
	})
}

func TestRoutingExchange_Head_AlwaysUsesCore(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(5)
	coreEx.head = headers[4]
	p2pEx.head = headers[4]

	blockTime := time.Second
	window := 100 * blockTime // high cutoff

	routingEx, err := NewRoutingExchange(coreEx, p2pEx,
		window,
		blockTime,
	)
	require.NoError(t, err)
	ctx := context.Background()

	// Head should always use core regardless of cutoff
	_, err = routingEx.Head(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_CalculateCutoffHeight(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(10)
	for _, h := range headers {
		coreEx.addHeader(h)
	}

	blockTime := time.Second
	window := 5 * time.Second

	routingEx, err := NewRoutingExchange(coreEx, p2pEx,
		window,
		blockTime,
	)
	require.NoError(t, err)

	t.Run("from inside window returns zero cutoff", func(t *testing.T) {
		// Header generated just now is inside a 5-second window
		fromHeader := headers[4] // height 5
		cutoff := routingEx.calculateCutoffHeight(fromHeader)
		assert.Equal(t, uint64(0), cutoff)
	})

	t.Run("from outside window calculates cutoff based on time", func(t *testing.T) {
		// Set header time to be outside the window
		fromHeader := headers[2] // height 3
		oldTime := time.Now().Add(-window - 3*blockTime)
		fromHeader.RawHeader.Time = oldTime

		cutoff := routingEx.calculateCutoffHeight(fromHeader)
		require.NoError(t, err)
		assert.Equal(t, uint64(6), cutoff)
	})

	t.Run("zero window returns error", func(t *testing.T) {
		_, err := NewRoutingExchange(coreEx, p2pEx, 0, blockTime)
		require.Error(t, err)
	})

	t.Run("zero blockTime returns error", func(t *testing.T) {
		_, err := NewRoutingExchange(coreEx, p2pEx, window, 0)
		require.Error(t, err)
	})
}

func TestRoutingExchange_Get_TriesCoreFirst(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(5)
	for _, h := range headers {
		coreEx.addHeader(h)
		p2pEx.addHeader(h)
	}

	routingEx, err := NewRoutingExchange(coreEx, p2pEx, time.Second, time.Second)
	require.NoError(t, err)

	ctx := context.Background()

	// Get by hash tries core first
	_, err = routingEx.Get(ctx, headers[2].Hash())
	require.NoError(t, err)
	assert.Equal(t, 1, coreEx.calls)
	assert.Equal(t, 0, p2pEx.calls)
}

func TestRoutingExchange_Get_FallsBackToP2P(t *testing.T) {
	coreEx := newMockExchange()
	p2pEx := newMockExchange()

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	headers := suite.GenExtendedHeaders(5)
	// Only add to p2p, not core
	for _, h := range headers {
		p2pEx.addHeader(h)
	}

	routingEx, err := NewRoutingExchange(coreEx, p2pEx, time.Second, time.Second)
	require.NoError(t, err)
	ctx := context.Background()

	// Get by hash should fall back to p2p if core fails
	_, err = routingEx.Get(ctx, headers[2].Hash())
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
	// `to` is exclusive, matching the libhead.Exchange contract.
	for i := from.Height() + 1; i < to; i++ {
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
