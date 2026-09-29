package shrex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/store"
)

func TestStreamedResponseReadFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	params := store.DefaultParameters()
	params.RecentBlocksCacheSize = 0
	dir := t.TempDir()
	s, err := store.NewStore(params, dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Stop(context.Background())) })
	ns := libshare.RandomNamespace()
	square, roots := edstest.RandEDSWithNamespace(t, ns, 16*16, 16)
	require.NoError(t, s.PutODSQ4(ctx, roots, 1, square))
	path := filepath.Join(dir, "blocks", "heights", "1.ods")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, os.Truncate(path, info.Size()/2))

	ch, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ch.Close()) })
	sh, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sh.Close()) })
	require.NoError(t, ch.Connect(ctx, peer.AddrInfo{ID: sh.ID(), Addrs: sh.Addrs()}))
	client, err := NewClient(DefaultClientParameters(), ch)
	require.NoError(t, err)
	server, err := NewServer(DefaultServerParameters(), sh, s)
	require.NoError(t, err)
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { require.NoError(t, server.Stop(context.Background())) })
	id, err := shwap.NewNamespaceDataID(1, ns)
	require.NoError(t, err)
	_, _, err = client.Get(ctx, &id, &shwap.NamespaceData{}, sh.ID())
	require.ErrorIs(t, err, ErrInternalServer)
	require.NotErrorIs(t, err, ErrInvalidResponse)
}
