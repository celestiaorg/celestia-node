package swamp

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/p2p"
)

var blackholeIP6 = net.ParseIP("100::")

// TODO(@Bidon15): Issue #350

// Swamp represents the main functionality that is needed for the test-case:
// - Network to link the nodes
// - CoreClient to share between Bridge nodes
// - Slices of created Bridge Nodes / Light Clients
// - trustedHash taken from the CoreClient and shared between nodes
type Swamp struct {
	t            *testing.T
	Network      mocknet.Mocknet
	CoreClient   core.Client
	BridgeNodes  []*node.Node
	LightClients []*node.Node
	trustedHash  string
}

// NewSwamp creates a new instance of Swamp.
func NewSwamp(t *testing.T) *Swamp {
	if testing.Verbose() {
		log.SetDebugLogging()
	}

	ctx := context.Background()
	// TODO(@Bidon15): Rework this to be configurable by the swamp's test
	// case, not here.
	kvStore := core.CreateKvStore(200)
	coreNode := core.StartMockNode(kvStore)

	coreNode.Config().Consensus.CreateEmptyBlocksInterval = 200 * time.Millisecond

	core := core.NewEmbeddedFromNode(coreNode)

	swp := &Swamp{t: t, Network: mocknet.New(ctx), CoreClient: core}
	swp.trustedHash = swp.getTrustedHash(ctx)

	return swp
}

// WaitTillHeight holds the test execution until the given amount of blocks
// have been produced by the CoreClient.
func (s *Swamp) WaitTillHeight(height int64) {
	require.Greater(s.t, height, int64(0))

	bf := core.NewBlockFetcher(s.CoreClient)
	blocks, err := bf.SubscribeNewBlockEvent(context.TODO())
	require.NoError(s.t, err)

	for {
		block := <-blocks
		if height == block.Height {
			break
		}
	}

	err = bf.UnsubscribeNewBlockEvent(context.TODO())
	require.NoError(s.t, err)
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
func (s *Swamp) getTrustedHash(ctx context.Context) string {
	bf := core.NewBlockFetcher(s.CoreClient)
	blocks, err := bf.SubscribeNewBlockEvent(ctx)
	require.NoError(s.t, err)
	block := <-blocks
	hstr := block.Hash().String()
	err = bf.UnsubscribeNewBlockEvent(ctx)
	require.NoError(s.t, err)
	return hstr
}

// TODO(@Bidon15): CoreClient(limitation)
// Now, we are making an assumption that consensus mechanism is already tested out
// so, we are not creating bridge nodes with each one containing its own core client
// instead we are assigning all created BNs to 1 Core from the swamp

// NewBridgeNode creates a new instance of BridgeNodes. Afterwards,
// the instance is store in the swamp's BridgeNodes slice.
func (s *Swamp) NewBridgeNode(options ...node.Option) *node.Node {
	cfg := node.DefaultConfig(node.Bridge)
	store := node.MockStore(s.t, cfg)

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

// NewLightClient creates a new instance of LightClient. Afterwards,
// the instance is store in the swamp's LightClients slice
func (s *Swamp) NewLightClient(options ...node.Option) *node.Node {
	cfg := node.DefaultConfig(node.Light)
	store := node.MockStore(s.t, cfg)

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
	s.LightClients = append(s.LightClients, node)
	return node
}
