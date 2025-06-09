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

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
	"github.com/celestiaorg/celestia-node/store"
)

func TestShrexFromLights(t *testing.T) {
	t.Parallel()
	const (
		blocks = 10
		btime  = time.Millisecond * 300
		bsize  = 16
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	heightsCh, fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)
	bridge := sw.NewBridgeNode()
	sw.SetBootstrapper(t, bridge)

	cfg := sw.DefaultTestConfig(node.Light)
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

	for i := range heightsCh {
		h, err := bridgeClient.Header.GetByHeight(ctx, i)
		require.NoError(t, err)

		var ns libshare.Namespace
		for _, roots := range h.DAH.RowRoots {
			// ensure to fetch random namespace (not the reserved namespace)
			namespace := roots[:libshare.NamespaceSize]
			ns, err = libshare.NewNamespaceFromBytes(namespace)
			require.NoError(t, err)
			if ns.IsUsableNamespace() {
				break
			}
		}

		if ns.ID() == nil {
			t.Fatal("usable namespace was not found")
		}

		height := h.Height()
		expected, err := bridgeClient.Share.GetNamespaceData(ctx, height, ns)
		require.NoError(t, err)
		_, err = lightClient.Header.WaitForHeight(ctx, height) // fixes a flake
		require.NoError(t, err)
		got, err := lightClient.Share.GetNamespaceData(ctx, height, ns)
		require.NoError(t, err)

		require.True(t, len(got[0].Shares) > 0)
		require.Equal(t, expected, got)

		expectedEds, err := bridgeClient.Share.GetEDS(ctx, height)
		require.NoError(t, err)

		gotEds, err := lightClient.Share.GetEDS(ctx, height)
		require.NoError(t, err)
		require.True(t, expectedEds.Equals(gotEds))
	}

	sw.StopNode(ctx, bridge)
	sw.StopNode(ctx, light)
}

func TestShrexFromLightsWithBadFulls(t *testing.T) {
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
	heightsCh, fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)

	bridge := sw.NewBridgeNode()
	sw.SetBootstrapper(t, bridge)

	// create full nodes with basic stream.reset handler
	ndHandler := func(stream network.Stream) {
		_ = stream.Reset()
	}
	fulls := make([]*nodebuilder.Node, 0, amountOfFulls)
	for i := 0; i < amountOfFulls; i++ {
		cfg := sw.DefaultTestConfig(node.Full)
		setTimeInterval(cfg, testTimeout)
		full := sw.NewNodeWithConfig(node.Full, cfg, replaceShrexServer(cfg, ndHandler), replaceShareGetter())
		fulls = append(fulls, full)
	}

	lnConfig := sw.DefaultTestConfig(node.Light)
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

	for i := range heightsCh {
		h, err := bridgeClient.Header.GetByHeight(ctx, i)
		require.NoError(t, err)

		var ns libshare.Namespace
		for _, roots := range h.DAH.RowRoots {
			// ensure to fetch random namespace (not the reserved namespace)
			namespace := roots[:libshare.NamespaceSize]
			ns, err = libshare.NewNamespaceFromBytes(namespace)
			require.NoError(t, err)
			if ns.IsUsableNamespace() {
				// found random data namespace
				break
			}
		}

		if ns.ID() == nil {
			t.Fatal("usable namespace was not found")
		}
		height := h.Height()

		expected, err := bridgeClient.Share.GetNamespaceData(ctx, height, ns)
		require.NoError(t, err)
		require.True(t, len(expected[0].Shares) > 0)

		// choose a random full to test
		fN := fulls[len(fulls)/2]
		fnClient := getAdminClient(ctx, fN, t)
		_, err = fnClient.Header.WaitForHeight(ctx, height) // fixes a flake
		require.NoError(t, err)
		gotFull, err := fnClient.Share.GetNamespaceData(ctx, height, ns)
		require.NoError(t, err)
		require.True(t, len(gotFull[0].Shares) > 0)

		_, err = lightClient.Header.WaitForHeight(ctx, height) // fixes a flake
		require.NoError(t, err)
		gotLight, err := lightClient.Share.GetNamespaceData(ctx, height, ns)
		require.NoError(t, err)
		require.True(t, len(gotLight[0].Shares) > 0)

		require.Equal(t, expected, gotFull)
		require.Equal(t, expected, gotLight)
	}

	sw.StopNode(ctx, bridge)
	stopFullNodes(ctx, sw, fulls...)
	sw.StopNode(ctx, light)
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

func stopFullNodes(ctx context.Context, sw *swamp.Swamp, fulls ...*nodebuilder.Node) {
	for _, full := range fulls {
		sw.StopNode(ctx, full)
	}
}

func replaceShrexServer(cfg *nodebuilder.Config, handler network.StreamHandler) fx.Option {
	return fx.Decorate(fx.Annotate(
		func(
			host host.Host,
			store *store.Store,
			network p2p.Network,
		) (*shrex.Server, error) {
			cfg.Share.ShrexServer.WithNetworkID(network.String())
			return shrex.NewServer(cfg.Share.ShrexServer, host, store, shrex.SupportedProtocols()...)
		},
		fx.OnStart(func(ctx context.Context, server *shrex.Server) error {
			for _, protocolName := range shrex.SupportedProtocols() {
				// replace handler for server
				server.SetHandler(
					shrex.ProtocolID(cfg.Share.ShrexServer.NetworkID(), protocolName.String()),
					handler,
				)
			}
			return nil
		}),
		fx.OnStop(func(ctx context.Context, server *shrex.Server) error {
			return server.Stop(ctx)
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
