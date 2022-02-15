package swamp

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	tn "github.com/tendermint/tendermint/node"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/p2p"
)

var blackholeIP6 = net.ParseIP("100::")

const subscriberID string = "Swamp"

var queryEvent string = types.QueryForEvent(types.EventNewBlock).String()

// Swamp represents the main functionality that is needed for the test-case:
// - Network to link the nodes
// - CoreClient to share between Bridge nodes
// - Slices of created Bridge/Light Nodes
// - trustedHash taken from the CoreClient and shared between nodes
type Swamp struct {
	t           *testing.T
	Network     mocknet.Mocknet
	CoreClient  core.Client
	BridgeNodes []*node.Node
	LightNodes  []*node.Node
	trustedHash string
}

// NewSwamp creates a new instance of Swamp.
func NewSwamp(t *testing.T, ic *InfraComps) *Swamp {
	if testing.Verbose() {
		log.SetDebugLogging()
	}

	var err error
	ctx := context.Background()

	tn, err := newTendermintCoreNode(ic)
	require.NoError(t, err)

	swp := &Swamp{
		t:          t,
		Network:    mocknet.New(ctx),
		CoreClient: core.NewEmbeddedFromNode(tn),
	}

	swp.trustedHash, err = swp.getTrustedHash(ctx)
	require.NoError(t, err)

	swp.t.Cleanup(func() {
		swp.stopAllNodes(ctx, swp.BridgeNodes, swp.LightNodes)
	})

	return swp
}

// TODO(@Bidon15): CoreClient(limitation)
// Now, we are making an assumption that consensus mechanism is already tested out
// so, we are not creating bridge nodes with each one containing its own core client
// instead we are assigning all created BNs to 1 Core from the swamp

// newTendermintCoreNode creates a new instance of Tendermint Core with a kvStore
func newTendermintCoreNode(ic *InfraComps) (*tn.Node, error) {
	var opt rpctest.Options
	rpctest.RecreateConfig(&opt)

	tn := rpctest.NewTendermint(ic.App, &opt)

	// rewriting the created config with test's one
	tn.Config().Consensus = ic.CoreCfg.Consensus

	return tn, tn.Start()
}

// stopAllNodes goes through all received slices of Nodes and stops one-by-one
// this eliminates a manual clean-up in the test-cases itself in the end
func (s *Swamp) stopAllNodes(ctx context.Context, allNodes ...[]*node.Node) {
	for _, nodes := range allNodes {
		for _, node := range nodes {
			require.NoError(s.t, node.Stop(ctx))
		}
	}
}

// WaitTillHeight holds the test execution until the given amount of blocks
// have been produced by the CoreClient.
func (s *Swamp) WaitTillHeight(ctx context.Context, height int64) {
	require.Greater(s.t, height, int64(0))
	results, err := s.CoreClient.Subscribe(ctx, subscriberID, queryEvent)
	require.NoError(s.t, err)

	defer func() {
		err = s.CoreClient.Unsubscribe(ctx, subscriberID, queryEvent)
		require.NoError(s.t, err)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case block := <-results:
			newBlock := block.Data.(types.EventDataNewBlock).Block
			if height == newBlock.Height {
				return
			}
		}
	}

}

// createPeer is a helper for celestia nodes to initialize
// with a real key instead of using a bogus one.
func (s *Swamp) createPeer(ks keystore.Keystore) host.Host {
	key, err := p2p.Key(ks)
	require.NoError(s.t, err)

	// IPv6 will be starting with 100:0
	token := make([]byte, 12)
	rand.Read(token) //nolint:gosec
	ip := append(net.IP{}, blackholeIP6...)
	copy(ip[net.IPv6len-len(token):], token)

	// reference to GenPeer func in libp2p/p2p/net/mock/mock_net.go
	// on how we generate new multiaddr for new peer
	a, err := ma.NewMultiaddr(fmt.Sprintf("/ip6/%s/tcp/4242", ip))
	require.NoError(s.t, err)

	host, err := s.Network.AddPeer(key, a)
	require.NoError(s.t, err)

	return host
}

// getTrustedHash is needed for celestia nodes to get the trustedhash
// from CoreClient. This is required to initialize and start correctly.
func (s *Swamp) getTrustedHash(ctx context.Context) (string, error) {
	results, err := s.CoreClient.Subscribe(ctx, subscriberID, queryEvent)
	require.NoError(s.t, err)

	defer func() {
		err = s.CoreClient.Unsubscribe(ctx, subscriberID, queryEvent)
		require.NoError(s.t, err)
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("can't get trusted hash as the channel is closed")
	case block := <-results:
		newBlock := block.Data.(types.EventDataNewBlock).Block
		return newBlock.Hash().String(), nil
	}
}

// NewBridgeNode creates a new instance of a BridgeNode providing a default config
// and a mockstore to the NewBridgeNodeWithStore method
func (s *Swamp) NewBridgeNode(options ...node.Option) *node.Node {
	cfg := node.DefaultConfig(node.Bridge)
	store := node.MockStore(s.t, cfg)

	return s.NewBridgeNodeWithStore(store, options...)
}

// NewLightNode creates a new instance of a LightNode providing a default config
// and a mockstore to the NewLightNodeWithStore method
func (s *Swamp) NewLightNode(options ...node.Option) *node.Node {
	cfg := node.DefaultConfig(node.Light)
	store := node.MockStore(s.t, cfg)

	return s.NewLightNodeWithStore(store, options...)
}

// NewBridgeNodeWithStore creates a new instance of BridgeNodes with predefined Store.
// Afterwards, the instance is stored in the swamp's BridgeNodes slice.
func (s *Swamp) NewBridgeNodeWithStore(store node.Store, options ...node.Option) *node.Node {
	ks, err := store.Keystore()
	require.NoError(s.t, err)

	// TODO(@Bidon15): If for some reason, we receive one of existing options
	// like <core, host, hash> from the test case, we need to check them and not use
	// default that are set here
	options = append(options,
		node.WithCoreClient(s.CoreClient),
		node.WithHost(s.createPeer(ks)),
		node.WithTrustedHash(s.trustedHash),
	)

	node, err := node.New(node.Bridge, store, options...)
	require.NoError(s.t, err)
	s.BridgeNodes = append(s.BridgeNodes, node)
	return node
}

// NewLightNodeWithStore creates a new instance of LightNode with predefined Store.
// Afterwards, the instance is stored in the swamp's LightNodes slice
func (s *Swamp) NewLightNodeWithStore(store node.Store, options ...node.Option) *node.Node {
	ks, err := store.Keystore()
	require.NoError(s.t, err)

	// TODO(@Bidon15): If for some reason, we receive one of existing options
	// like <core, host, hash> from the test case, we need to check them and not use
	// default that are set here
	options = append(options,
		node.WithHost(s.createPeer(ks)),
		node.WithTrustedHash(s.trustedHash),
	)

	node, err := node.New(node.Light, store, options...)
	require.NoError(s.t, err)
	s.LightNodes = append(s.LightNodes, node)

	return node
}
