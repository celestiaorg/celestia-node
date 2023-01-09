package discovery

import (
	"context"
	"testing"
	"time"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestSet_TryAdd(t *testing.T) {
	m := mocknet.New()
	h, err := m.GenPeer()
	require.NoError(t, err)

	set := newLimitedSet(1)
	require.NoError(t, set.TryAdd(h.ID()))
	require.True(t, set.Contains(h.ID()))
}

func TestSet_TryAddFails(t *testing.T) {
	m := mocknet.New()
	h1, err := m.GenPeer()
	require.NoError(t, err)
	h2, err := m.GenPeer()
	require.NoError(t, err)

	set := newLimitedSet(1)
	require.NoError(t, set.TryAdd(h1.ID()))
	require.Error(t, set.TryAdd(h2.ID()))
}

func TestSet_Remove(t *testing.T) {
	m := mocknet.New()
	h, err := m.GenPeer()
	require.NoError(t, err)

	set := newLimitedSet(1)
	require.NoError(t, set.TryAdd(h.ID()))
	set.Remove(h.ID())
	require.False(t, set.Contains(h.ID()))
}

func TestSet_Peers(t *testing.T) {
	m := mocknet.New()
	h1, err := m.GenPeer()
	require.NoError(t, err)
	h2, err := m.GenPeer()
	require.NoError(t, err)

	set := newLimitedSet(2)
	require.NoError(t, set.TryAdd(h1.ID()))
	require.NoError(t, set.TryAdd(h2.ID()))

	peers := set.ListPeers()
	require.True(t, len(peers) == 2)
}

// TestSet_WaitPeers ensures that `WaitPeers` will be unblocked once
// a new peer was discovered.
func TestSet_WaitPeers(t *testing.T) {
	m := mocknet.New()
	h1, err := m.GenPeer()
	require.NoError(t, err)

	set := newLimitedSet(2)
	go func() {
		time.Sleep(time.Millisecond * 500)
		set.TryAdd(h1.ID()) //nolint:errcheck
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	t.Cleanup(cancel)

	// call `WaitPeer` on empty set will block until a new peer will be discovered
	peers, err := set.WaitPeer(ctx)
	require.NoError(t, err)
	require.True(t, len(peers) == 1)
}

func TestSet_Size(t *testing.T) {
	m := mocknet.New()
	h1, err := m.GenPeer()
	require.NoError(t, err)
	h2, err := m.GenPeer()
	require.NoError(t, err)

	set := newLimitedSet(2)
	require.NoError(t, set.TryAdd(h1.ID()))
	require.NoError(t, set.TryAdd(h2.ID()))
	require.Equal(t, 2, set.Size())
	set.Remove(h2.ID())
	require.Equal(t, 1, set.Size())
}
