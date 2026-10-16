package shrex

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/store"
)

func TestExchange_RequestND_NotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	edsStore, client, server := makeExchange(t)

	height := atomic.Uint64{}
	height.Add(1)

	t.Run("CAR_not_exist", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		namespace := libshare.RandomNamespace()
		height := height.Add(1)

		id, err := shwap.NewNamespaceDataID(height, namespace)
		data := shwap.NamespaceData{}
		require.NoError(t, err)
		_, _, err = client.Get(ctx, &id, &data, server.host.ID())
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("ErrNamespaceNotFound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eds := edstest.RandEDS(t, 4)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)

		height := height.Add(1)
		err = edsStore.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)

		namespace := libshare.RandomNamespace()

		id, err := shwap.NewNamespaceDataID(height, namespace)
		data := shwap.NamespaceData{}
		require.NoError(t, err)

		resp := delayedResponse{ReaderFrom: &data, delay: 20 * time.Millisecond}
		_, payloadDuration, err := client.Get(ctx, &id, &resp, server.host.ID())
		require.NoError(t, err)
		require.GreaterOrEqual(t, payloadDuration, resp.delay)
		require.Empty(t, data.Flatten())
	})
}

// Regression for #3480: canceling ctx mid-request must abort promptly instead
// of hanging on the stream deadline.
func TestClient_AbortsOnCtxCancel(t *testing.T) {
	hosts := createMocknet(t, 2)
	client, err := NewClient(DefaultClientParameters(), hosts[0])
	require.NoError(t, err)

	id, err := shwap.NewNamespaceDataID(1, libshare.RandomNamespace())
	require.NoError(t, err)

	// Server never reads or writes, so the client stays blocked in serde.Read
	// until the fix's AfterFunc resets the stream.
	serverBlocked := make(chan struct{})
	unblock := make(chan struct{})
	t.Cleanup(func() { close(unblock) })
	hosts[1].SetStreamHandler(
		ProtocolID(client.params.NetworkID(), id.Name()),
		func(s network.Stream) {
			defer s.Reset() //nolint:errcheck
			close(serverBlocked)
			<-unblock
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)
	go func() {
		_, _, err := client.Get(ctx, &id, &shwap.NamespaceData{}, hosts[1].ID())
		result <- err
	}()

	select {
	case <-serverBlocked:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received the stream")
	}

	cancel()

	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("client did not return promptly after context cancellation")
	}
}

type delayedResponse struct {
	io.ReaderFrom
	delay time.Duration
}

func (r *delayedResponse) ReadFrom(reader io.Reader) (int64, error) {
	time.Sleep(r.delay)
	return r.ReaderFrom.ReadFrom(reader)
}

func createMocknet(t *testing.T, amount int) []libhost.Host {
	t.Helper()

	net, err := mocknet.FullMeshConnected(amount)
	require.NoError(t, err)
	// get host and peer
	return net.Hosts()
}

func makeExchange(t *testing.T) (*store.Store, *Client, *Server) {
	t.Helper()
	s, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	hosts := createMocknet(t, 2)

	client, err := NewClient(DefaultClientParameters(), hosts[0])
	require.NoError(t, err)
	server, err := NewServer(DefaultServerParameters(), hosts[1], s)
	require.NoError(t, err)
	err = server.WithMetrics()
	require.NoError(t, err)
	require.NoError(t, server.Start(context.Background()))
	return s, client, server
}

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
