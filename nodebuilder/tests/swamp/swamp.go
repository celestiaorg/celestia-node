package swamp

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/testutil/testnode"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/logs"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	coremodule "github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/share/eds"
)

var blackholeIP6 = net.ParseIP("100::")

// DefaultTestTimeout should be used as the default timout on all the Swamp tests.
// It's generously set to 5 minutes to give enough time for CI.
const DefaultTestTimeout = time.Minute * 5

// Swamp represents the main functionality that is needed for the test-case:
// - Network to link the nodes
// - CoreClient to share between Bridge nodes
// - Slices of created Bridge/Full/Light Nodes
// - trustedHash taken from the CoreClient and shared between nodes
type Swamp struct {
	t           *testing.T
	Network     mocknet.Mocknet
	BridgeNodes []*nodebuilder.Node
	FullNodes   []*nodebuilder.Node
	LightNodes  []*nodebuilder.Node
	comps       *Components

	ClientContext testnode.Context
	Accounts      []string

	genesis *header.ExtendedHeader
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

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Now, we are making an assumption that consensus mechanism is already tested out
	// so, we are not creating bridge nodes with each one containing its own core client
	// instead we are assigning all created BNs to 1 Core from the swamp
	cctx := core.StartTestNodeWithConfig(t, ic.TestConfig)
	swp := &Swamp{
		t:             t,
		Network:       mocknet.New(),
		ClientContext: cctx,
		comps:         ic,
		Accounts:      ic.Accounts,
	}

	swp.t.Cleanup(func() {
		swp.stopAllNodes(ctx, swp.BridgeNodes, swp.FullNodes, swp.LightNodes)
	})

	swp.setupGenesis(ctx)
	return swp
}

// stopAllNodes goes through all received slices of Nodes and stops one-by-one
// this eliminates a manual clean-up in the test-cases itself in the end
func (s *Swamp) stopAllNodes(ctx context.Context, allNodes ...[]*nodebuilder.Node) {
	for _, nodes := range allNodes {
		for _, node := range nodes {
			require.NoError(s.t, node.Stop(ctx))
		}
	}
}

// GetCoreBlockHashByHeight returns a tendermint block's hash by provided height
func (s *Swamp) GetCoreBlockHashByHeight(ctx context.Context, height int64) libhead.Hash {
	b, err := s.ClientContext.Client.Block(ctx, &height)
	require.NoError(s.t, err)
	return libhead.Hash(b.BlockID.Hash)
}

// WaitTillHeight holds the test execution until the given amount of blocks
// has been produced by the CoreClient.
func (s *Swamp) WaitTillHeight(ctx context.Context, height int64) libhead.Hash {
	require.Greater(s.t, height, int64(0))

	t := time.NewTicker(time.Millisecond * 50)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			require.NoError(s.t, ctx.Err())
		case <-t.C:
			status, err := s.ClientContext.Client.Status(ctx)
			require.NoError(s.t, err)

			latest := status.SyncInfo.LatestBlockHeight
			switch {
			case latest == height:
				return libhead.Hash(status.SyncInfo.LatestBlockHash)
			case latest > height:
				res, err := s.ClientContext.Client.Block(ctx, &height)
				require.NoError(s.t, err)
				return libhead.Hash(res.BlockID.Hash)
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

// setupGenesis sets up genesis Header.
// This is required to initialize and start correctly.
func (s *Swamp) setupGenesis(ctx context.Context) {
	s.WaitTillHeight(ctx, 1)

	store, err := eds.NewStore(s.t.TempDir(), ds_sync.MutexWrap(ds.NewMapDatastore()))
	require.NoError(s.t, err)

	ex := core.NewExchange(
		core.NewBlockFetcher(s.ClientContext.Client),
		store,
		header.MakeExtendedHeader,
	)

	h, err := ex.GetByHeight(ctx, 1)
	require.NoError(s.t, err)
	s.genesis = h
}

// NewBridgeNode creates a new instance of a BridgeNode providing a default config
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewBridgeNode(options ...fx.Option) *nodebuilder.Node {
	cfg := nodebuilder.DefaultConfig(node.Bridge)
	store := nodebuilder.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Bridge, store, options...)
}

// NewFullNode creates a new instance of a FullNode providing a default config
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewFullNode(options ...fx.Option) *nodebuilder.Node {
	cfg := nodebuilder.DefaultConfig(node.Full)
	store := nodebuilder.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Full, store, options...)
}

// NewLightNode creates a new instance of a LightNode providing a default config
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewLightNode(options ...fx.Option) *nodebuilder.Node {
	cfg := nodebuilder.DefaultConfig(node.Light)
	store := nodebuilder.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Light, store, options...)
}

func (s *Swamp) NewNodeWithConfig(nodeType node.Type, cfg *nodebuilder.Config, options ...fx.Option) *nodebuilder.Node {
	store := nodebuilder.MockStore(s.t, cfg)
	return s.NewNodeWithStore(nodeType, store, options...)
}

// NewNodeWithStore creates a new instance of Node with predefined Store.
// Afterwards, the instance is stored in the swamp's Nodes' slice according to the
// node's type provided from the user.
func (s *Swamp) NewNodeWithStore(
	t node.Type,
	store nodebuilder.Store,
	options ...fx.Option,
) *nodebuilder.Node {
	var n *nodebuilder.Node

	options = append(options,
		state.WithKeyringSigner(nodebuilder.TestKeyringSigner(s.t)),
	)

	switch t {
	case node.Bridge:
		options = append(options,
			coremodule.WithClient(s.ClientContext.Client),
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

func (s *Swamp) newNode(t node.Type, store nodebuilder.Store, options ...fx.Option) *nodebuilder.Node {
	ks, err := store.Keystore()
	require.NoError(s.t, err)

	// TODO(@Bidon15): If for some reason, we receive one of existing options
	// like <core, host, hash> from the test case, we need to check them and not use
	// default that are set here
	cfg, _ := store.Config()
	cfg.RPC.Port = "0"

	// tempDir is used for the eds.Store
	tempDir := s.t.TempDir()
	options = append(options,
		p2p.WithHost(s.createPeer(ks)),
		fx.Replace(node.StorePath(tempDir)),
		fx.Invoke(func(ctx context.Context, store libhead.Store[*header.ExtendedHeader]) error {
			return store.Init(ctx, s.genesis)
		}),
	)
	node, err := nodebuilder.New(t, p2p.Private, store, options...)
	require.NoError(s.t, err)
	return node
}

// RemoveNode removes a node from the swamp's node slice
// this allows reusage of the same var in the test scenario
// if the user needs to stop and start the same node
func (s *Swamp) RemoveNode(n *nodebuilder.Node, t node.Type) error {
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

func (s *Swamp) remove(rn *nodebuilder.Node, sn []*nodebuilder.Node) ([]*nodebuilder.Node, error) {
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

// Disconnect allows to break a connection between two peers without any possibility to
// re-establish it. Order is very important here. We have to unlink peers first, and only after
// that call disconnect. This is hard disconnect and peers will not be able to reconnect.
// In order to reconnect peers again, please use swamp.Connect
func (s *Swamp) Disconnect(t *testing.T, peerA, peerB peer.ID) {
	require.NoError(t, s.Network.UnlinkPeers(peerA, peerB))
	require.NoError(t, s.Network.DisconnectPeers(peerA, peerB))
}
