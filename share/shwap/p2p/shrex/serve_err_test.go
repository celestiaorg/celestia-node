package shrex

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
	"github.com/celestiaorg/celestia-node/store"
)

// realHosts returns two connected loopback hosts. Mocknet drops stream reset codes, so tests that
// depend on them need a real transport.
func realHosts(t *testing.T) (libhost.Host, libhost.Host) {
	t.Helper()
	newHost := func() libhost.Host {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, h.Close()) })
		return h
	}
	client, server := newHost(), newHost()
	err := client.Connect(context.Background(), peer.AddrInfo{ID: server.ID(), Addrs: server.Addrs()})
	require.NoError(t, err)
	return client, server
}

// TestClient_ServeErrIsNotInvalidResponse checks that a server resetting with streamServeErr after
// OK is reported as an internal server error, not as an invalid response the peer gets
// blacklisted for, while a plain truncated response still is invalid.
func TestClient_ServeErrIsNotInvalidResponse(t *testing.T) {
	for name, tc := range map[string]struct {
		end      func(network.Stream)
		wantErr  error
		wantNoIs error
	}{
		"reset with serve error": {
			end:      func(s network.Stream) { s.ResetWithError(streamServeErr) }, //nolint:errcheck
			wantErr:  ErrInternalServer,
			wantNoIs: ErrInvalidResponse,
		},
		"truncated and closed": {
			end:     func(s network.Stream) { s.Close() },
			wantErr: ErrInvalidResponse,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)

			clientHost, serverHost := realHosts(t)
			client, err := NewClient(DefaultClientParameters(), clientHost)
			require.NoError(t, err)

			id, err := shwap.NewNamespaceDataID(1, libshare.RandomNamespace())
			require.NoError(t, err)

			serverHost.SetStreamHandler(ProtocolID(client.params.NetworkID(), id.Name()), func(s network.Stream) {
				var req shwap.NamespaceDataID
				if _, err := req.ReadFrom(s); err != nil {
					s.Reset() //nolint:errcheck
					return
				}
				if _, err := serde.Write(s, &shrexpb.Response{Status: shrexpb.Status_OK}); err != nil {
					s.Reset() //nolint:errcheck
					return
				}
				// a partial row: the start of a length-delimited message that never completes.
				if _, err := s.Write([]byte{0xff, 0x01, 0x0a}); err != nil {
					s.Reset() //nolint:errcheck
					return
				}
				tc.end(s)
			})

			_, _, err = client.Get(ctx, &id, &shwap.NamespaceData{}, serverHost.ID())
			require.ErrorIs(t, err, tc.wantErr)
			if tc.wantNoIs != nil {
				require.NotErrorIs(t, err, tc.wantNoIs)
			}
		})
	}
}

// TestServer_ResetsWithServeErrOnReadFailure checks the server end to end: when reading the
// response fails after OK was sent, the client sees an internal server error instead of a
// truncated response it would blacklist the server for.
func TestServer_ResetsWithServeErrOnReadFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	params := store.DefaultParameters()
	// without the recent cache every request opens the file from disk.
	params.RecentBlocksCacheSize = 0
	dir := t.TempDir()
	s, err := store.NewStore(params, dir)
	require.NoError(t, err)

	const (
		odsSize = 16
		height  = 1
	)
	namespace := libshare.RandomNamespace()
	randEDS, roots := edstest.RandEDSWithNamespace(t, namespace, odsSize*odsSize, odsSize)
	require.NoError(t, s.PutODSQ4(ctx, roots, height, randEDS))

	// cut the ODS file in half: roots and the first rows stay readable, later rows fail.
	path := filepath.Join(dir, "blocks", "heights", strconv.Itoa(height)+".ods")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, os.Truncate(path, info.Size()/2))

	clientHost, serverHost := realHosts(t)
	client, err := NewClient(DefaultClientParameters(), clientHost)
	require.NoError(t, err)
	server, err := NewServer(DefaultServerParameters(), serverHost, s)
	require.NoError(t, err)
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { require.NoError(t, server.Stop(context.Background())) })

	rowIdxs, err := share.RowsWithNamespace(roots, namespace)
	require.NoError(t, err)
	require.Len(t, rowIdxs, odsSize)

	id, err := shwap.NewNamespaceDataID(height, namespace)
	require.NoError(t, err)
	_, _, err = client.Get(ctx, &id, &shwap.NamespaceData{}, serverHost.ID())
	require.ErrorIs(t, err, ErrInternalServer)
	require.NotErrorIs(t, err, ErrInvalidResponse)
}
