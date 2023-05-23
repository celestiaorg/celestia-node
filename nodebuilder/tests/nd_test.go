package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

func TestShrexNDFromLights(t *testing.T) {
	const (
		blocks = 5
		btime  = time.Millisecond * 300
		bsize  = 16
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)

	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	bridge := sw.NewBridgeNode()
	addrsBridge, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	os.Setenv(p2p.EnvKeyCelestiaBootstrapper, "true")
	const defaultTimeInterval = time.Second * 5

	light := newLightNode(sw, addrsBridge[0].String(), defaultTimeInterval, 1)

	require.NoError(t, bridge.Start(ctx))
	require.NoError(t, startLightNodes(ctx, light))
	require.NoError(t, <-fillDn)

	for i := 1; i < blocks; i++ {
		h, err := bridge.HeaderServ.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		nID := h.DAH.RowsRoots[0][:8]
		reqCtx, cancel := context.WithTimeout(ctx, time.Second*5)
		sh, err := light.ShareServ.GetSharesByNamespace(reqCtx, h.DAH, nID)
		cancel()
		require.NoError(t, err)
		require.True(t, len(sh[0].Shares) > 0)
	}
}

func TestShrexNDFromLightsWithBadFulls(t *testing.T) {
	const (
		blocks              = 5
		btime               = time.Millisecond * 300
		bsize               = 16
		amountOfFulls       = 25
		defaultTimeInterval = time.Second * 5
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	bridge := sw.NewBridgeNode()
	addrsBridge, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	os.Setenv(p2p.EnvKeyCelestiaBootstrapper, "true")

	ndHandler := func(stream network.Stream) {
		_ = stream.Reset()
	}

	// create full nodes with basic reset handler
	fulls := make([]*nodebuilder.Node, 0, amountOfFulls)
	for i := 0; i < amountOfFulls; i++ {
		fulls = append(fulls, newFullNodeWithNDHandler(sw, addrsBridge[0].String(), defaultTimeInterval, ndHandler))
	}

	light := newLightNode(sw, addrsBridge[0].String(), defaultTimeInterval, amountOfFulls+1)

	require.NoError(t, bridge.Start(ctx))
	require.NoError(t, startFullNodes(ctx, fulls...))
	require.NoError(t, startLightNodes(ctx, light))
	require.NoError(t, <-fillDn)

	for i := 1; i < blocks; i++ {
		h, err := bridge.HeaderServ.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		nID := h.DAH.RowsRoots[0][:8]
		reqCtx, cancel := context.WithTimeout(ctx, time.Second*5)
		sh, err := light.ShareServ.GetSharesByNamespace(reqCtx, h.DAH, nID)
		cancel()
		require.NoError(t, err)
		require.True(t, len(sh[0].Shares) > 0)
	}
}

func newLightNode(
	sw *swamp.Swamp,
	bootstarpperAddr string,
	defaultTimeInterval time.Duration,
	amountOfFulls int,
) *nodebuilder.Node {
	lnConfig := nodebuilder.DefaultConfig(node.Light)
	lnConfig.Share.UseIPLD = false
	lnConfig.Share.Discovery.PeersLimit = uint(amountOfFulls)
	setTimeInterval(lnConfig, defaultTimeInterval)
	lnConfig.Header.TrustedPeers = append(lnConfig.Header.TrustedPeers, bootstarpperAddr)
	return sw.NewNodeWithConfig(node.Light, lnConfig)
}

func startLightNodes(ctx context.Context, nodes ...*nodebuilder.Node) error {
	for _, node := range nodes {
		sub, err := node.Host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
		if err != nil {
			return err
		}

		if err = node.Start(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sub.Out():
			if err = sub.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func newFullNodeWithNDHandler(
	sw *swamp.Swamp,
	bootstarpperAddr string,
	defaultTimeInterval time.Duration,
	handler network.StreamHandler,
) *nodebuilder.Node {
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Share.UseIPLD = false
	setTimeInterval(cfg, defaultTimeInterval)
	cfg.Header.TrustedPeers = []string{
		"/ip4/1.2.3.4/tcp/12345/p2p/12D3KooWNaJ1y1Yio3fFJEXCZyd1Cat3jmrPdgkYCrHfKD3Ce21p",
	}
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, bootstarpperAddr)

	return sw.NewNodeWithConfig(node.Full, cfg, replaceNDServer(cfg, handler))
}

func startFullNodes(ctx context.Context, fulls ...*nodebuilder.Node) error {
	for _, full := range fulls {
		err := full.Start(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func replaceNDServer(cfg *nodebuilder.Config, handler network.StreamHandler) fx.Option {
	return fx.Decorate(fx.Annotate(
		func(
			host host.Host,
			store *eds.Store,
			getter *getters.StoreGetter,
			network p2p.Network,
		) (*shrexnd.Server, error) {
			cfg.Share.ShrExNDParams.WithNetworkID(network.String())
			return shrexnd.NewServer(cfg.Share.ShrExNDParams, host, store, getter)
		},
		fx.OnStart(func(ctx context.Context, server *shrexnd.Server) error {
			// replace handler for server
			server.SetHandler(handler)
			return server.Start(ctx)
		}),
		fx.OnStop(func(ctx context.Context, server *shrexnd.Server) error {
			return server.Start(ctx)
		}),
	))
}
