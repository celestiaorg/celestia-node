package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestBackoff_ConnectPeer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	m, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	b := newBackoffConnector(m.Hosts()[0], backoff.NewFixedBackoff(time.Minute))
	info := host.InfoFromHost(m.Hosts()[1])
	require.NoError(t, b.Connect(ctx, *info))
}

func TestBackoff_ConnectPeerFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	m, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	b := newBackoffConnector(m.Hosts()[0], backoff.NewFixedBackoff(time.Minute))
	info := host.InfoFromHost(m.Hosts()[1])
	require.NoError(t, b.Connect(ctx, *info))

	require.Error(t, b.Connect(ctx, *info))
}

func TestBackoff_ResetBackoffPeriod(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	m, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	b := newBackoffConnector(m.Hosts()[0], backoff.NewFixedBackoff(time.Minute))
	info := host.InfoFromHost(m.Hosts()[1])
	require.NoError(t, b.Connect(ctx, *info))
	nexttry := b.cacheData[info.ID].nexttry
	b.Backoff(info.ID)
	require.True(t, b.cacheData[info.ID].nexttry.After(nexttry))
}
