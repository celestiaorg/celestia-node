package canary

import (
	"context"
	"errors"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

type nativeFixture struct {
	failedSession
	rpc                   *rpcclient.Client
	infoCalls, peersCalls int
}

func (s *nativeFixture) RPC(context.Context) (*rpcclient.Client, error) { return s.rpc, nil }
func (s *nativeFixture) P2PInfo(context.Context) (peer.AddrInfo, error) {
	s.infoCalls++
	return peer.AddrInfo{ID: peer.ID("metadata-only")}, nil
}

func (s *nativeFixture) P2PPeers(context.Context) ([]peer.ID, error) {
	s.peersCalls++
	return []peer.ID{peer.ID("bootstrap")}, nil
}

func TestAdaptSessionUsesMetadataMethodsNotReadTokenP2P(t *testing.T) {
	native := &nativeFixture{rpc: &rpcclient.Client{}}
	native.rpc.P2P.Internal.Info = func(context.Context) (peer.AddrInfo, error) {
		t.Error("read-token P2P.Info forbidden")
		return peer.AddrInfo{}, errors.New("admin required")
	}
	native.rpc.P2P.Internal.Peers = func(context.Context) ([]peer.ID, error) {
		t.Error("read-token P2P.Peers forbidden")
		return nil, errors.New("admin required")
	}
	native.rpc.DAS.Internal.SamplingStats = func(context.Context) (
		das.SamplingStats,
		error,
	) {
		return das.SamplingStats{SampledChainHead: 7}, nil
	}
	r, err := AdaptSession(native).RPC(context.Background())
	require.NoError(t, err)
	pi, err := r.Info(context.Background())
	require.NoError(t, err)
	require.Equal(t, peer.ID("metadata-only"), pi.ID)
	peers, err := r.Peers(context.Background())
	require.NoError(t, err)
	require.Equal(t, []peer.ID{peer.ID("bootstrap")}, peers)
	st, err := r.SamplingStats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 7, st.SampledChainHead)
	require.Equal(t, 1, native.infoCalls)
	require.Equal(t, 1, native.peersCalls)
	require.NoError(t, r.Close())
}

func TestAdaptSessionForwardsOnlyNativeHeaderReads(t *testing.T) {
	native := &nativeFixture{rpc: &rpcclient.Client{}}
	hs := headertest.NewTestSuite(t).GenExtendedHeaders(2)
	calls := 0
	native.rpc.Header.Internal.NetworkHead = func(context.Context) (
		*header.ExtendedHeader,
		error,
	) {
		calls++
		return hs[1], nil
	}
	native.rpc.Header.Internal.Tail = func(context.Context) (
		*header.ExtendedHeader,
		error,
	) {
		calls++
		return hs[0], nil
	}
	native.rpc.Header.Internal.GetByHeight = func(_ context.Context, h uint64) (*header.ExtendedHeader, error) {
		calls++
		require.Equal(t, hs[1].Height(), h)
		return hs[1], nil
	}
	native.rpc.Header.Internal.GetByHash = func(_ context.Context, h libhead.Hash) (*header.ExtendedHeader, error) {
		calls++
		require.Equal(t, hs[0].Hash(), h)
		return hs[0], nil
	}
	native.rpc.Header.Internal.GetRangeByHeight = func(
		_ context.Context,
		from *header.ExtendedHeader,
		to uint64,
	) ([]*header.ExtendedHeader, error) {
		calls++
		require.Same(t, hs[0], from)
		require.Equal(t, hs[1].Height()+1, to)
		return hs[1:], nil
	}
	ctx := context.Background()
	r, err := AdaptSession(native).RPC(ctx)
	require.NoError(t, err)
	h, err := r.NetworkHead(ctx)
	require.NoError(t, err)
	require.Same(t, hs[1], h)
	h, err = r.Tail(ctx)
	require.NoError(t, err)
	require.Same(t, hs[0], h)
	h, err = r.GetByHeight(ctx, hs[1].Height())
	require.NoError(t, err)
	require.Same(t, hs[1], h)
	h, err = r.GetByHash(ctx, hs[0].Hash())
	require.NoError(t, err)
	require.Same(t, hs[0], h)
	batch, err := r.GetRangeByHeight(ctx, hs[0], hs[1].Height()+1)
	require.NoError(t, err)
	require.Equal(t, hs[1:], batch)
	require.Equal(t, 5, calls)
	require.NoError(t, r.Close())
	require.NoError(t, r.Close())
	native.rpc = nil
	_, err = AdaptSession(native).RPC(ctx)
	require.Error(t, err)
	require.Nil(t, AdaptSession(nil))
}
