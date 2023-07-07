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
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

func TestShrexNDFromLights(t *testing.T) {
	const (
		blocks      = 10
		btime       = time.Millisecond * 300
		bsize       = 16
		testTimeout = time.Second * 10
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)

	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	bridge := sw.NewBridgeNode()
	addrsBridge, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	os.Setenv(p2p.EnvKeyCelestiaBootstrapper, "true")

	light := newLightNode(sw, addrsBridge[0].String(), testTimeout, 1)
	require.NoError(t, bridge.Start(ctx))
	require.NoError(t, startLightNodes(ctx, light))
	require.NoError(t, <-fillDn)

	// first 2 blocks are not filled with data
	for i := 3; i < blocks; i++ {
		h, err := bridge.HeaderServ.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		namespace := h.DAH.RowRoots[0][:share.NamespaceSize]
		reqCtx, cancel := context.WithTimeout(ctx, time.Second*5)
		sh, err := light.ShareServ.GetSharesByNamespace(reqCtx, h.DAH, namespace)
		cancel()
		require.NoError(t, err)
		require.True(t, len(sh[0].Shares) > 0)
	}
}

func TestShrexNDFromLightsWithBadFulls(t *testing.T) {
	const (
		blocks        = 10
		btime         = time.Millisecond * 300
		bsize         = 16
		amountOfFulls = 5
		testTimeout   = time.Second * 10
	)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	bridge := sw.NewBridgeNode()
	addrsBridge, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	os.Setenv(p2p.EnvKeyCelestiaBootstrapper, "true")

	// create full nodes with basic stream.reset handler
	ndHandler := func(stream network.Stream) {
		_ = stream.Reset()
	}
	fulls := make([]*nodebuilder.Node, 0, amountOfFulls)
	for i := 0; i < amountOfFulls; i++ {
		full := newFullNodeWithNDHandler(
			sw,
			addrsBridge[0].String(),
			testTimeout,
			ndHandler)

		fulls = append(fulls, full)
	}

	light := newLightNode(sw, addrsBridge[0].String(), testTimeout, amountOfFulls+1)
	require.NoError(t, bridge.Start(ctx))
	require.NoError(t, startFullNodes(ctx, fulls...))
	require.NoError(t, startLightNodes(ctx, light))
	require.NoError(t, <-fillDn)

	// first 2 blocks are not filled with data
	for i := 3; i < blocks; i++ {
		h, err := bridge.HeaderServ.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		namespace := h.DAH.RowRoots[0][:share.NamespaceSize]
		reqCtx, cancel := context.WithTimeout(ctx, time.Second*5)
		sh, err := light.ShareServ.GetSharesByNamespace(reqCtx, h.DAH, namespace)
		cancel()
		require.NoError(t, err)
		require.True(t, len(sh[0].Shares) > 0)
	}
}

func newLightNode(
	sw *swamp.Swamp,
	bootstrapperAddr string,
	defaultTimeInterval time.Duration,
	amountOfFulls int,
) *nodebuilder.Node {
	lnConfig := nodebuilder.DefaultConfig(node.Light)
	lnConfig.Share.Discovery.PeersLimit = uint(amountOfFulls)
	setTimeInterval(lnConfig, defaultTimeInterval)
	lnConfig.Header.TrustedPeers = append(lnConfig.Header.TrustedPeers, bootstrapperAddr)
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
	bootstrapperAddr string,
	defaultTimeInterval time.Duration,
	handler network.StreamHandler,
) *nodebuilder.Node {
	cfg := nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)
	cfg.Header.TrustedPeers = []string{
		"/ip4/1.2.3.4/tcp/12345/p2p/12D3KooWNaJ1y1Yio3fFJEXCZyd1Cat3jmrPdgkYCrHfKD3Ce21p",
	}
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, bootstrapperAddr)

	return sw.NewNodeWithConfig(node.Full, cfg, replaceNDServer(cfg, handler), replaceShareGetter())
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

func replaceShareGetter() fx.Option {
	return fx.Decorate(fx.Annotate(
		func(
			host host.Host,
			store *eds.Store,
			storeGetter *getters.StoreGetter,
			shrexGetter *getters.ShrexGetter,
			network p2p.Network,
		) share.Getter {
			cascade := make([]share.Getter, 0, 2)
			cascade = append(cascade, storeGetter)
			cascade = append(cascade, getters.NewTeeGetter(shrexGetter, store))
			return getters.NewCascadeGetter(cascade)
		},
	))
}
