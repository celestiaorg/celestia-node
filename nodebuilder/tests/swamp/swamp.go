package swamp

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/fx"
	"golang.org/x/exp/maps"

	"github.com/celestiaorg/celestia-app/test/util/testnode"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
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
	t   *testing.T
	cfg *testnode.Config

	Network       mocknet.Mocknet
	Bootstrappers []ma.Multiaddr

	ClientContext testnode.Context
	Accounts      []string

	nodesMu sync.Mutex
	nodes   map[*nodebuilder.Node]struct{}

	genesis *header.ExtendedHeader
}

// NewSwamp creates a new instance of Swamp.
func NewSwamp(t *testing.T, options ...Option) *Swamp {
	if testing.Verbose() {
		logs.SetDebugLogging()
	}

	ic := DefaultConfig()
	for _, option := range options {
		option(ic)
	}

	// Now, we are making an assumption that consensus mechanism is already tested out
	// so, we are not creating bridge nodes with each one containing its own core client
	// instead we are assigning all created BNs to 1 Core from the swamp
	cctx := core.StartTestNodeWithConfig(t, ic)
	swp := &Swamp{
		t:             t,
		cfg:           ic,
		Network:       mocknet.New(),
		ClientContext: cctx,
		Accounts:      ic.Accounts,
		nodes:         map[*nodebuilder.Node]struct{}{},
	}

	swp.t.Cleanup(swp.cleanup)
	swp.setupGenesis()
	return swp
}

// cleanup frees up all the resources
// including stop of all created nodes
func (s *Swamp) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(s.t, s.Network.Close())

	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()
	maps.DeleteFunc(s.nodes, func(nd *nodebuilder.Node, _ struct{}) bool {
		require.NoError(s.t, nd.Stop(ctx))
		return true
	})
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
			latest, err := s.ClientContext.LatestHeight()
			require.NoError(s.t, err)
			if latest >= height {
				res, err := s.ClientContext.Client.Block(ctx, &latest)
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
	_, _ = rand.Read(token)
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
func (s *Swamp) setupGenesis() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// ensure core has surpassed genesis block
	s.WaitTillHeight(ctx, 2)

	ds := ds_sync.MutexWrap(ds.NewMapDatastore())
	store, err := eds.NewStore(eds.DefaultParameters(), s.t.TempDir(), ds)
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

// DefaultTestConfig creates a test config with the access to the core node for the tp
func (s *Swamp) DefaultTestConfig(tp node.Type) *nodebuilder.Config {
	cfg := nodebuilder.DefaultConfig(tp)

	ip, port, err := net.SplitHostPort(s.cfg.AppConfig.GRPC.Address)
	require.NoError(s.t, err)

	cfg.Core.IP = ip
	cfg.Core.GRPCPort = port
	return cfg
}

// NewBridgeNode creates a new instance of a BridgeNode providing a default config
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewBridgeNode(options ...fx.Option) *nodebuilder.Node {
	cfg := s.DefaultTestConfig(node.Bridge)
	store := nodebuilder.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Bridge, store, options...)
}

// NewFullNode creates a new instance of a FullNode providing a default config
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewFullNode(options ...fx.Option) *nodebuilder.Node {
	cfg := s.DefaultTestConfig(node.Full)
	cfg.Header.TrustedPeers = []string{
		"/ip4/1.2.3.4/tcp/12345/p2p/12D3KooWNaJ1y1Yio3fFJEXCZyd1Cat3jmrPdgkYCrHfKD3Ce21p",
	}
	// add all bootstrappers in suite as trusted peers
	for _, bootstrapper := range s.Bootstrappers {
		cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, bootstrapper.String())
	}
	store := nodebuilder.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Full, store, options...)
}

// NewLightNode creates a new instance of a LightNode providing a default config
// and a mockstore to the NewNodeWithStore method
func (s *Swamp) NewLightNode(options ...fx.Option) *nodebuilder.Node {
	cfg := s.DefaultTestConfig(node.Light)
	cfg.Header.TrustedPeers = []string{
		"/ip4/1.2.3.4/tcp/12345/p2p/12D3KooWNaJ1y1Yio3fFJEXCZyd1Cat3jmrPdgkYCrHfKD3Ce21p",
	}
	// add all bootstrappers in suite as trusted peers
	for _, bootstrapper := range s.Bootstrappers {
		cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, bootstrapper.String())
	}

	store := nodebuilder.MockStore(s.t, cfg)

	return s.NewNodeWithStore(node.Light, store, options...)
}

func (s *Swamp) NewNodeWithConfig(nodeType node.Type, cfg *nodebuilder.Config, options ...fx.Option) *nodebuilder.Node {
	store := nodebuilder.MockStore(s.t, cfg)
	// add all bootstrappers in suite as trusted peers
	for _, bootstrapper := range s.Bootstrappers {
		cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, bootstrapper.String())
	}
	return s.NewNodeWithStore(nodeType, store, options...)
}

// NewNodeWithStore creates a new instance of Node with predefined Store.
func (s *Swamp) NewNodeWithStore(
	tp node.Type,
	store nodebuilder.Store,
	options ...fx.Option,
) *nodebuilder.Node {
	signer := apptypes.NewKeyringSigner(s.ClientContext.Keyring, s.Accounts[0], s.ClientContext.ChainID)
	options = append(options,
		state.WithKeyringSigner(signer),
	)

	switch tp {
	case node.Bridge:
		options = append(options,
			coremodule.WithClient(s.ClientContext.Client),
		)
	default:
	}

	nd := s.newNode(tp, store, options...)
	s.nodesMu.Lock()
	s.nodes[nd] = struct{}{}
	s.nodesMu.Unlock()
	return nd
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

// StopNode stops the node and removes from Swamp.
// TODO(@Wondertan): For clean and symmetrical API, we may want to add StartNode.
func (s *Swamp) StopNode(ctx context.Context, nd *nodebuilder.Node) {
	s.nodesMu.Lock()
	delete(s.nodes, nd)
	s.nodesMu.Unlock()
	require.NoError(s.t, nd.Stop(ctx))
}

// Connect allows to connect peers after hard disconnection.
func (s *Swamp) Connect(t *testing.T, peerA, peerB *nodebuilder.Node) {
	_, err := s.Network.LinkPeers(peerA.Host.ID(), peerB.Host.ID())
	require.NoError(t, err)
	_, err = s.Network.ConnectPeers(peerA.Host.ID(), peerB.Host.ID())
	require.NoError(t, err)
}

// Disconnect allows to break a connection between two peers without any possibility to
// re-establish it. Order is very important here. We have to unlink peers first, and only after
// that call disconnect. This is hard disconnect and peers will not be able to reconnect.
// In order to reconnect peers again, please use swamp.Connect
func (s *Swamp) Disconnect(t *testing.T, peerA, peerB *nodebuilder.Node) {
	require.NoError(t, s.Network.UnlinkPeers(peerA.Host.ID(), peerB.Host.ID()))
	require.NoError(t, s.Network.DisconnectPeers(peerA.Host.ID(), peerB.Host.ID()))
}

// SetBootstrapper sets the given bootstrappers as the "bootstrappers" for the
// Swamp test suite. Every new full or light node created on the suite afterwards
// will automatically add the suite's bootstrappers as trusted peers to their config.
// NOTE: Bridge nodes do not automaatically add the bootstrappers as trusted peers.
// NOTE: Use `NewNodeWithStore` to avoid this automatic configuration.
func (s *Swamp) SetBootstrapper(t *testing.T, bootstrappers ...*nodebuilder.Node) {
	for _, trusted := range bootstrappers {
		addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(trusted.Host))
		require.NoError(t, err)
		s.Bootstrappers = append(s.Bootstrappers, addrs[0])
	}
}

// Validators retrieves keys from the app node in order to build the validators.
func (s *Swamp) Validators(t *testing.T) (*types.ValidatorSet, types.PrivValidator) {
	privPath := s.cfg.TmConfig.PrivValidatorKeyFile()
	statePath := s.cfg.TmConfig.PrivValidatorStateFile()
	priv := privval.LoadFilePV(privPath, statePath)
	key, err := priv.GetPubKey()
	require.NoError(t, err)
	validator := types.NewValidator(key, 100)
	set := types.NewValidatorSet([]*types.Validator{validator})
	return set, priv
}
