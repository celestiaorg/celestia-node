package discovery

import (
	"testing"

	"github.com/stretchr/testify/require"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
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
	require.True(t, len(set.Peers()) == 2)
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
