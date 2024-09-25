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
	set.Add(h.ID())
	require.True(t, set.Contains(h.ID()))
}

func TestSet_Remove(t *testing.T) {
	m := mocknet.New()
	h, err := m.GenPeer()
	require.NoError(t, err)

	set := newLimitedSet(1)
	set.Add(h.ID())
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
	set.Add(h1.ID())
	set.Add(h2.ID())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancel)

	peers, err := set.Peers(ctx)
	require.NoError(t, err)
	require.True(t, len(peers) == 2)
}

// TestSet_WaitPeers ensures that `Peers` will be unblocked once
// a new peer was discovered.
func TestSet_WaitPeers(t *testing.T) {
	m := mocknet.New()
	h1, err := m.GenPeer()
	require.NoError(t, err)

	set := newLimitedSet(2)
	go func() {
		time.Sleep(time.Millisecond * 500)
		set.Add(h1.ID())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	t.Cleanup(cancel)

	// call `Peers` on empty set will block until a new peer will be discovered
	peers, err := set.Peers(ctx)
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
	set.Add(h1.ID())
	set.Add(h2.ID())
	require.EqualValues(t, 2, set.Size())
	set.Remove(h2.ID())
	require.EqualValues(t, 1, set.Size())
}
