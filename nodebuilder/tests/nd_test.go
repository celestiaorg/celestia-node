//go:build nd || integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd"
	"github.com/celestiaorg/celestia-node/store"
)

func TestShrexNDFromLights(t *testing.T) {
	const (
		blocks = 10
		btime  = time.Millisecond * 300
		bsize  = 16
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)

	bridge := sw.NewBridgeNode()
	sw.SetBootstrapper(t, bridge)

	cfg := nodebuilder.DefaultConfig(node.Light)
	cfg.Share.Discovery.PeersLimit = 1
	light := sw.NewNodeWithConfig(node.Light, cfg)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	err = light.Start(ctx)
	require.NoError(t, err)

	bridgeClient := getAdminClient(ctx, bridge, t)
	lightClient := getAdminClient(ctx, light, t)

	// wait for chain to be filled
	require.NoError(t, <-fillDn)

	// first 15 blocks are not filled with data
	//
	// TODO: we need to stop guessing
	// the block that actually has transactions. We can get this data from the
	// response returned by FillBlock.
	for i := 16; i < blocks; i++ {
		h, err := bridgeClient.Header.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		reqCtx, cancel := context.WithTimeout(ctx, time.Second*5)

		// ensure to fetch random namespace (not the reserved namespace)
		namespace := h.DAH.RowRoots[1][:share.NamespaceSize]

		expected, err := bridgeClient.Share.GetSharesByNamespace(reqCtx, h, namespace)
		require.NoError(t, err)
		got, err := lightClient.Share.GetSharesByNamespace(reqCtx, h, namespace)
		require.NoError(t, err)

		require.True(t, len(got[0].Shares) > 0)
		require.Equal(t, expected, got)

		cancel()
	}
}

func TestShrexNDFromLightsWithBadFulls(t *testing.T) {
	const (
		blocks        = 10
		btime         = time.Millisecond * 300
		bsize         = 16
		amountOfFulls = 5
		testTimeout   = time.Second * 20
	)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)

	bridge := sw.NewBridgeNode()
	sw.SetBootstrapper(t, bridge)

	// create full nodes with basic stream.reset handler
	ndHandler := func(stream network.Stream) {
		_ = stream.Reset()
	}
	fulls := make([]*nodebuilder.Node, 0, amountOfFulls)
	for i := 0; i < amountOfFulls; i++ {
		cfg := nodebuilder.DefaultConfig(node.Full)
		setTimeInterval(cfg, testTimeout)
		full := sw.NewNodeWithConfig(node.Full, cfg, replaceNDServer(cfg, ndHandler), replaceShareGetter())
		fulls = append(fulls, full)
	}

	lnConfig := nodebuilder.DefaultConfig(node.Light)
	lnConfig.Share.Discovery.PeersLimit = uint(amountOfFulls)
	light := sw.NewNodeWithConfig(node.Light, lnConfig)

	// start all nodes
	require.NoError(t, bridge.Start(ctx))
	require.NoError(t, startFullNodes(ctx, fulls...))
	require.NoError(t, light.Start(ctx))

	bridgeClient := getAdminClient(ctx, bridge, t)
	lightClient := getAdminClient(ctx, light, t)

	// wait for chain to fill up
	require.NoError(t, <-fillDn)

	// first 2 blocks are not filled with data
	for i := 3; i < blocks; i++ {
		h, err := bridgeClient.Header.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		if len(h.DAH.RowRoots) != bsize*2 {
			// fill blocks does not always fill every block to the given block
			// size - this check prevents trying to fetch shares for the parity
			// namespace.
			continue
		}

		reqCtx, cancel := context.WithTimeout(ctx, time.Second*5)

		// ensure to fetch random namespace (not the reserved namespace)
		namespace := h.DAH.RowRoots[1][:share.NamespaceSize]

		expected, err := bridgeClient.Share.GetSharesByNamespace(reqCtx, h, namespace)
		require.NoError(t, err)
		require.True(t, len(expected[0].Shares) > 0)

		// choose a random full to test
		fN := fulls[len(fulls)/2]
		fnClient := getAdminClient(ctx, fN, t)
		gotFull, err := fnClient.Share.GetSharesByNamespace(reqCtx, h, namespace)
		require.NoError(t, err)
		require.True(t, len(gotFull[0].Shares) > 0)

		gotLight, err := lightClient.Share.GetSharesByNamespace(reqCtx, h, namespace)
		require.NoError(t, err)
		require.True(t, len(gotLight[0].Shares) > 0)

		require.Equal(t, expected, gotFull)
		require.Equal(t, expected, gotLight)

		cancel()
	}
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
			store *store.Store,
			network p2p.Network,
		) (*shrexnd.Server, error) {
			cfg.Share.ShrExNDParams.WithNetworkID(network.String())
			return shrexnd.NewServer(cfg.Share.ShrExNDParams, host, store)
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
			store *store.Store,
			storeGetter *store.Getter,
			shrexGetter *shrex_getter.Getter,
			network p2p.Network,
		) shwap.Getter {
			cascade := make([]shwap.Getter, 0, 2)
			cascade = append(cascade, storeGetter)
			cascade = append(cascade, shrexGetter)
			return getters.NewCascadeGetter(cascade)
		},
	))
}
