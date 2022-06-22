package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

func TestSet_TryAdd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer t.Cleanup(cancel)
	m := mocknet.New(ctx)
	h, err := m.GenPeer()
	require.NoError(t, err)

	set := NewLimitedSet(1)
	require.NoError(t, set.TryAdd(h.ID()))
	require.True(t, set.Contains(h.ID()))
}

func TestSet_TryAddFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer t.Cleanup(cancel)
	m := mocknet.New(ctx)
	h1, err := m.GenPeer()
	require.NoError(t, err)
	h2, err := m.GenPeer()
	require.NoError(t, err)

	set := NewLimitedSet(1)
	require.NoError(t, set.TryAdd(h1.ID()))
	require.Error(t, set.TryAdd(h2.ID()))
}

func TestSet_Remove(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer t.Cleanup(cancel)
	m := mocknet.New(ctx)
	h, err := m.GenPeer()
	require.NoError(t, err)

	set := NewLimitedSet(1)
	require.NoError(t, set.TryAdd(h.ID()))
	set.Remove(h.ID())
	require.False(t, set.Contains(h.ID()))
}

func TestSet_Peers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer t.Cleanup(cancel)
	m := mocknet.New(ctx)
	h1, err := m.GenPeer()
	require.NoError(t, err)
	h2, err := m.GenPeer()
	require.NoError(t, err)

	set := NewLimitedSet(2)
	require.NoError(t, set.TryAdd(h1.ID()))
	require.NoError(t, set.TryAdd(h2.ID()))
	require.Equal(t, []peer.ID{h1.ID(), h2.ID()}, set.Peers())
}

func TestSet_Size(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer t.Cleanup(cancel)
	m := mocknet.New(ctx)
	h1, err := m.GenPeer()
	require.NoError(t, err)
	h2, err := m.GenPeer()
	require.NoError(t, err)

	set := NewLimitedSet(2)
	require.NoError(t, set.TryAdd(h1.ID()))
	require.NoError(t, set.TryAdd(h2.ID()))
	require.Equal(t, 2, set.Size())
	set.Remove(h2.ID())
	require.Equal(t, 1, set.Size())
}
