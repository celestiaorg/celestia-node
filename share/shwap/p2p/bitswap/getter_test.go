package bitswap

import (
	"context"
	"sync"
	"testing"

	"github.com/ipfs/boxo/exchange"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

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
	return &mockFetcher{id: m.sessionCount, ctx: ctx}
}

// mockFetcher is a mock implementation of exchange.Fetcher
type mockFetcher struct {
	exchange.Fetcher
	id  int
	ctx context.Context
}

func TestPoolGetFromEmptyPool(t *testing.T) {
	ex := &mockSessionExchange{}
	p := newPool(ex)
	ctx := context.Background()
	p.ctx = ctx

	ses, release := p.get()
	defer release()
	fetcher := ses.(*mockFetcher)
	require.NotNil(t, fetcher)
	require.Equal(t, 1, fetcher.id)
}

func TestPoolPutAndGet(t *testing.T) {
	ex := &mockSessionExchange{}
	p := newPool(ex)
	ctx := context.Background()
	p.ctx = ctx

	// Get a session
	ses, release := p.get()

	// Put it back
	release()

	// Get again
	ses2, release2 := p.get()
	defer release2()

	require.Equal(t, ses.(*mockFetcher).id, ses2.(*mockFetcher).id)
}

func TestPoolConcurrency(t *testing.T) {
	ex := &mockSessionExchange{}
	p := newPool(ex)
	ctx := context.Background()
	p.ctx = ctx

	const numGoroutines = 50
	var wg sync.WaitGroup

	// Start multiple goroutines to get sessions
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, release := p.get()
			release()
		}()
	}
	wg.Wait()

	require.LessOrEqual(t, len(p.sessions), maxIdleSessions)
}

func TestPoolBoundsIdleSessions(t *testing.T) {
	ex := &mockSessionExchange{}
	p := newPool(ex)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p.ctx = ctx

	sessions := make([]exchange.Fetcher, maxIdleSessions+1)
	releases := make([]func(), maxIdleSessions+1)
	for i := range sessions {
		sessions[i], releases[i] = p.get()
	}
	for _, release := range releases {
		release()
	}

	require.Len(t, p.sessions, maxIdleSessions)
	require.ErrorIs(t, sessions[maxIdleSessions].(*mockFetcher).ctx.Err(), context.Canceled)
	for _, ses := range sessions[:maxIdleSessions] {
		require.NoError(t, ses.(*mockFetcher).ctx.Err())
	}

	_, release := p.get()
	release()
	require.Equal(t, maxIdleSessions+1, ex.sessionCount)
}

func TestPoolDiscardsSessionReleasedAfterStop(t *testing.T) {
	ex := &mockSessionExchange{}
	p := newPool(ex)
	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx

	ses, release := p.get()
	cancel()
	release()

	require.Empty(t, p.sessions)
	require.ErrorIs(t, ses.(*mockFetcher).ctx.Err(), context.Canceled)
}
