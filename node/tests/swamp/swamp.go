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
	"github.com/tendermint/tendermint/libs/bytes"
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
// - Slices of created Bridge/Full/Light Nodes
// - trustedHash taken from the CoreClient and shared between nodes
type Swamp struct {
	t           *testing.T
	Network     mocknet.Mocknet
	CoreClient  core.Client
	BridgeNodes []*node.Node
	FullNodes   []*node.Node
	LightNodes  []*node.Node
	trustedHash string
}

// NewSwamp creates a new instance of Swamp.
func NewSwamp(t *testing.T, ic *Components) *Swamp {
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
func newTendermintCoreNode(c *Components) (*tn.Node, error) {
	var opt rpctest.Options
	rpctest.RecreateConfig(&opt)

	tn := rpctest.NewTendermint(c.App, &opt)

	// rewriting the created config with test's one
	tn.Config().Consensus = c.CoreCfg.Consensus

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

// GetCoreBlockHashByHeight returns a tendermint block's hash by provided height
func (s *Swamp) GetCoreBlockHashByHeight(ctx context.Context, height int64) bytes.HexBytes {
	b, err := s.CoreClient.Block(ctx, &height)
	require.NoError(s.t, err)
	return b.BlockID.Hash
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

	require.NoError(s.t, s.Network.LinkAll())
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

// NewFullNode creates a new instance of a FullNode providing a default config
// and a mockstore to the NewFullNodeWithStore method
func (s *Swamp) NewFullNode(options ...node.Option) *node.Node {
	cfg := node.DefaultConfig(node.Full)
	store := node.MockStore(s.t, cfg)

	return s.NewFullNodeWithStore(store, options...)
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

// NewFullNodeWithStore creates a new instance of FullNode with predefined Store.
// Afterwards, the instance is stored in the swamp's FullNodes slice
func (s *Swamp) NewFullNodeWithStore(store node.Store, options ...node.Option) *node.Node {
	ks, err := store.Keystore()
	require.NoError(s.t, err)

	// TODO(@Bidon15): If for some reason, we receive one of existing options
	// like <core, host, hash> from the test case, we need to check them and not use
	// default that are set here
	options = append(options,
		node.WithHost(s.createPeer(ks)),
		node.WithTrustedHash(s.trustedHash),
	)

	node, err := node.New(node.Full, store, options...)
	require.NoError(s.t, err)
	s.FullNodes = append(s.FullNodes, node)
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

// RemoveNode removes a node from the swamp's node slice
// this allows reusage of the same var in the test scenario
// if the user needs to stop and start the same node
func (s *Swamp) RemoveNode(n *node.Node, t node.Type) error {
	var err error
	switch t {
	case node.Light:
		s.LightNodes, err = s.remove(n, s.LightNodes)
		return err
	case node.Bridge:
		s.BridgeNodes, err = s.remove(n, s.BridgeNodes)
		return err
	case node.Full:
		s.FullNodes, err = s.remove(n, s.FullNodes)
		return err
	default:
		return fmt.Errorf("no such type or node")
	}
}

func (s *Swamp) remove(rn *node.Node, sn []*node.Node) ([]*node.Node, error) {
	if len(sn) == 1 {
		return nil, nil
	}

	initSize := len(sn)
	for i := 0; i < len(sn); i++ {
		if sn[i] == rn {
			sn = append(sn[:i], sn[i+1:]...)
			i--
		}
	}

	if initSize <= len(sn) {
		return sn, fmt.Errorf("cannot delete the node")
	}
	return sn, nil
}
