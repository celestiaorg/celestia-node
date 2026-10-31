package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

type countingDiscovery struct {
	findPeers int
}

func (c *countingDiscovery) Advertise(context.Context, string, ...discovery.Option) (time.Duration, error) {
	return 0, nil
}

func (c *countingDiscovery) FindPeers(context.Context, string, ...discovery.Option) (<-chan peer.AddrInfo, error) {
	c.findPeers++
	ch := make(chan peer.AddrInfo)
	close(ch)
	return ch, nil
}

// TestDiscoverSkipsWhenAboveLimit checks that a set that overshot its limit (concurrent
// workers can add more peers than wanted) is treated as full instead of wrapping the
// unsigned "want" count around.
func TestDiscoverSkipsWhenAboveLimit(t *testing.T) {
	params := DefaultParameters()
	params.PeersLimit = 3
	disc := &countingDiscovery{}
	d, err := NewDiscovery(params, nil, disc, "full", "v1")
	require.NoError(t, err)

	for i := range 5 {
		d.set.Add(peer.ID(rune('a' + i)))
	}
	require.Greater(t, d.set.Size(), d.set.Limit())

	require.True(t, d.discover(context.Background()))
	require.Zero(t, disc.findPeers)
}
