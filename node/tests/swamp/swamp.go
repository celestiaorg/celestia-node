package swamp

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/logs"
	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/params"
)

var blackholeIP6 = net.ParseIP("100::")

const subscriberID string = "NewBlockSwamp/Events"

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
	comps       *Components
}

// NewSwamp creates a new instance of Swamp.
func NewSwamp(t *testing.T, options ...Option) *Swamp {
	if testing.Verbose() {
		logs.SetDebugLogging()
	}

	ic := DefaultComponents()
	for _, option := range options {
		option(ic)
	}

	var err error
	ctx := context.Background()

	// TODO(@Bidon15): CoreClient(limitation)
	// Now, we are making an assumption that consensus mechanism is already tested out
	// so, we are not creating bridge nodes with each one containing its own core client
	// instead we are assigning all created BNs to 1 Core from the swamp
	core.StartTestNode(ctx, t, ic.App, ic.CoreCfg)
	endpoint, err := core.GetEndpoint(ic.CoreCfg)
	require.NoError(t, err)
	ip, port, err := net.SplitHostPort(endpoint)
	require.NoError(t, err)
	remote, err := core.NewRemote(ip, port)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := remote.Stop()
		require.NoError(t, err)
	})

	err = remote.Start()
	require.NoError(t, err)

	swp := &Swamp{
		t:          t,
		Network:    mocknet.New(),
		CoreClient: remote,
		comps:      ic,
	}

	swp.trustedHash, err = swp.getTrustedHash(ctx)
	require.NoError(t, err)

	swp.t.Cleanup(func() {
		swp.stopAllNodes(ctx, swp.BridgeNodes, swp.FullNodes, swp.LightNodes)
	})

	return swp
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
// has been produced by the CoreClient.
func (s *Swamp) WaitTillHeight(ctx context.Context, height int64) {
	require.Greater(s.t, height, int64(0))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results, err := s.CoreClient.Subscribe(ctx, subscriberID, queryEvent)
	require.NoError(s.t, err)

	defer func() {
		// TODO(@Wondertan): For some reason, the Unsubscribe does not work and we have to do
		//  an UnsubscribeAll as a hack. There is somewhere a bug in the Tendermint which should be
		//  investigated
		err = s.CoreClient.UnsubscribeAll(ctx, subscriberID)
		require.NoError(s.t, err)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case block := <-results:
			newBlock := block.Data.(types.EventDataNewBlock)
			if height <= newBlock.Block.Height {
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results, err := s.CoreClient.Subscribe(ctx, subscriberID, queryEvent)
	require.NoError(s.t, err)

	defer func() {
		err := s.CoreClient.UnsubscribeAll(ctx, subscriberID)
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
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewBridgeNode(options ...node.Option) *node.Node {
	cfg := node.DefaultConfig(node.Bridge)
	store := node.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Bridge, store, options...)
}

// NewFullNode creates a new instance of a FullNode providing a default config
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewFullNode(options ...node.Option) *node.Node {
	cfg := node.DefaultConfig(node.Full)
	store := node.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Full, store, options...)
}

// NewLightNode creates a new instance of a LightNode providing a default config
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewLightNode(options ...node.Option) *node.Node {
	cfg := node.DefaultConfig(node.Light)
	store := node.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Light, store, options...)
}

// NewNodeWithStore creates a new instance of Node with predefined Store.
// Afterwards, the instance is stored in the swamp's Nodes' slice according to the
// node's type provided from the user.
func (s *Swamp) NewNodeWithStore(t node.Type, store node.Store, options ...node.Option) *node.Node {
	var n *node.Node

	options = append(options, node.WithKeyringSigner(node.TestKeyringSigner(s.t)))

	switch t {
	case node.Bridge:
		options = append(options,
			node.WithCoreClient(s.CoreClient),
		)
		n = s.newNode(node.Bridge, store, options...)
		s.BridgeNodes = append(s.BridgeNodes, n)
	case node.Full:
		n = s.newNode(node.Full, store, options...)
		s.FullNodes = append(s.FullNodes, n)
	case node.Light:
		n = s.newNode(node.Light, store, options...)
		s.LightNodes = append(s.LightNodes, n)
	}

	return n
}

func (s *Swamp) newNode(t node.Type, store node.Store, options ...node.Option) *node.Node {
	ks, err := store.Keystore()
	require.NoError(s.t, err)

	// TODO(@Bidon15): If for some reason, we receive one of existing options
	// like <core, host, hash> from the test case, we need to check them and not use
	// default that are set here
	options = append(options,
		node.WithHost(s.createPeer(ks)),
		node.WithTrustedHash(s.trustedHash),
		node.WithNetwork(params.Private),
		node.WithRPCPort("0"),
	)

	node, err := node.New(t, store, options...)
	require.NoError(s.t, err)

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

// Connect allows to connect peers after hard disconnection.
func (s *Swamp) Connect(t *testing.T, peerA, peerB peer.ID) {
	_, err := s.Network.LinkPeers(peerA, peerB)
	require.NoError(t, err)
	_, err = s.Network.ConnectPeers(peerA, peerB)
	require.NoError(t, err)
}

// Disconnect allows to break a connection between two peers without any possibility to re-establish it.
// Order is very important here. We have to unlink peers first, and only after that call disconnect.
// This is hard disconnect and peers will not be able to reconnect.
// In order to reconnect peers again, please use swamp.Connect
func (s *Swamp) Disconnect(t *testing.T, peerA, peerB peer.ID) {
	require.NoError(t, s.Network.UnlinkPeers(peerA, peerB))
	require.NoError(t, s.Network.DisconnectPeers(peerA, peerB))
}
