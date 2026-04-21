package bitswap

import (
	"context"
	"sync"
	"testing"

	"github.com/ipfs/boxo/exchange"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestEDSFromRows(t *testing.T) {
	edsIn := edstest.RandEDS(t, 8)
	roots, err := share.NewAxisRoots(edsIn)
	require.NoError(t, err)

	rows := make([]shwap.Row, edsIn.Width()/2)
	for i := range edsIn.Width() / 2 {
		rowShrs, err := libshare.FromBytes(edsIn.Row(i)[:edsIn.Width()/2])
		require.NoError(t, err)
		rows[i] = shwap.NewRow(rowShrs, shwap.Left)
	}

	edsOut, err := edsFromRows(roots, rows)
	require.NoError(t, err)
	require.True(t, edsIn.Equals(edsOut))
}

// mockSessionExchange is a mock implementation of exchange.SessionExchange
type mockSessionExchange struct {
	exchange.SessionExchange
	sessionCount int
	mu           sync.Mutex
}

func (m *mockSessionExchange) NewSession(ctx context.Context) exchange.Fetcher {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionCount++
	return &mockFetcher{id: m.sessionCount}
}

// mockFetcher is a mock implementation of exchange.Fetcher
type mockFetcher struct {
	exchange.Fetcher
	id int
}

func TestPoolGetFromEmptyPool(t *testing.T) {
	ex := &mockSessionExchange{}
	p := newPool(ex)
	ctx := context.Background()
	p.ctx = ctx

	ses := p.get().(*mockFetcher)
	require.NotNil(t, ses)
	require.Equal(t, 1, ses.id)
}

func TestPoolPutAndGet(t *testing.T) {
	ex := &mockSessionExchange{}
	p := newPool(ex)
	ctx := context.Background()
	p.ctx = ctx

	// Get a session
	ses := p.get().(*mockFetcher)

	// Put it back
	p.put(ses)

	// Get again
	ses2 := p.get().(*mockFetcher)

	require.Equal(t, ses.id, ses2.id)
}

func TestPoolConcurrency(t *testing.T) {
	ex := &mockSessionExchange{}
	p := newPool(ex)
	ctx := context.Background()
	p.ctx = ctx

	const numGoroutines = 50
	var wg sync.WaitGroup

	sessionIDSet := make(map[int]struct{})
	lock := sync.Mutex{}

	// Start multiple goroutines to get sessions
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ses := p.get()
			mockSes := ses.(*mockFetcher)
			p.put(ses)
			lock.Lock()
			sessionIDSet[mockSes.id] = struct{}{}
			lock.Unlock()
		}()
	}
	wg.Wait()

	// Since the pool reuses sessions, the number of unique session IDs should be less than or equal to numGoroutines
	if len(sessionIDSet) > numGoroutines {
		t.Fatalf("expected number of unique sessions to be less than or equal to %d, got %d",
			numGoroutines, len(sessionIDSet))
	}
}
