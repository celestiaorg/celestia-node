package swamp

import (
	"context"
	"crypto/rand"
	"maps"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/v6/test/util/testnode"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/logs"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// DefaultTestTimeout should be used as the default timeout on all the Swamp tests.
// It's generously set to 10 minutes to give enough time for CI.
const DefaultTestTimeout = time.Minute * 10

// Swamp represents the main functionality that is needed for the test-case:
// - Network to link the nodes
// - CoreClient to share between Bridge nodes
// - Slices of created Bridge/Light Nodes
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
	ic.WithChainID("private")
	cctx := core.StartTestNodeWithConfig(t, ic)
	swp := &Swamp{
		t:             t,
		cfg:           ic,
		Network:       mocknet.New(),
		ClientContext: cctx,
		Accounts:      getAccounts(ic),
		nodes:         map[*nodebuilder.Node]struct{}{},
	}

	swp.t.Cleanup(swp.cleanup)
	swp.WaitTillHeight(context.Background(), 1)
	return swp
}

func getAccounts(config *testnode.Config) (accounts []string) {
	for _, account := range config.Genesis.Accounts() {
		accounts = append(accounts, account.Name)
	}
	return accounts
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

	addr := &net.TCPAddr{
		IP:   make(net.IP, 4),
		Port: 4242,
	}
	rand.Read(addr.IP) //nolint:errcheck

	ma, err := manet.FromNetAddr(addr)
	require.NoError(s.t, err)

	host, err := s.Network.AddPeer(key, ma)
	require.NoError(s.t, err)

	require.NoError(s.t, s.Network.LinkAll())
	return host
}

// DefaultTestConfig creates a test config with the access to the core node for the tp
func (s *Swamp) DefaultTestConfig(tp node.Type) *nodebuilder.Config {
	cfg := nodebuilder.DefaultConfig(tp)

	ip, port, err := net.SplitHostPort(s.cfg.AppConfig.GRPC.Address)
	require.NoError(s.t, err)
	cfg.Core.IP = ip
	cfg.Core.Port = port
	// set port to zero so that OS allocates one
	// this avoids port collissions between nodes and tests
	cfg.RPC.Port = "0"

	for _, bootstrapper := range s.Bootstrappers {
		cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, bootstrapper.String())
	}
	return cfg
}

// NewBridgeNode creates a new instance of a BridgeNode providing a default config
// and a mockstore to the MustNewNodeWithStore method
func (s *Swamp) NewBridgeNode(options ...fx.Option) *nodebuilder.Node {
	cfg := s.DefaultTestConfig(node.Bridge)
	return s.NewNodeWithConfig(node.Bridge, cfg, options...)
}

// NewLightNode creates a new instance of a LightNode providing a default config
// and a mockstore to the MustNewNodeWithStore method
func (s *Swamp) NewLightNode(options ...fx.Option) *nodebuilder.Node {
	cfg := s.DefaultTestConfig(node.Light)
	return s.NewNodeWithConfig(node.Light, cfg, options...)
}

func (s *Swamp) NewNodeWithConfig(nodeType node.Type, cfg *nodebuilder.Config, options ...fx.Option) *nodebuilder.Node {
	store := nodebuilder.MockStore(s.t, cfg)
	return s.MustNewNodeWithStore(nodeType, store, options...)
}

// MustNewNodeWithStore creates a new instance of Node with predefined Store.
func (s *Swamp) MustNewNodeWithStore(
	tp node.Type,
	store nodebuilder.Store,
	options ...fx.Option,
) *nodebuilder.Node {
	nd, err := s.NewNodeWithStore(tp, store, options...)
	require.NoError(s.t, err)
	return nd
}

// NewNodeWithStore creates a new instance of Node with predefined Store.
func (s *Swamp) NewNodeWithStore(
	tp node.Type,
	store nodebuilder.Store,
	options ...fx.Option,
) (*nodebuilder.Node, error) {
	nd, err := s.newNode(tp, store, options...)
	if err != nil {
		return nil, err
	}

	s.nodesMu.Lock()
	s.nodes[nd] = struct{}{}
	s.nodesMu.Unlock()
	return nd, nil
}

func (s *Swamp) newNode(t node.Type, store nodebuilder.Store, options ...fx.Option) (*nodebuilder.Node, error) {
	ks, err := store.Keystore()
	if err != nil {
		return nil, err
	}

	options = append(
		options,
		p2p.WithHost(s.createPeer(ks)),
		fx.Replace(node.StorePath(s.t.TempDir())),
		state.WithKeyring(s.ClientContext.Keyring),
		state.WithKeyName(state.AccountName(s.Accounts[0])),
	)
	return nodebuilder.New(t, p2p.Private, store, options...)
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
// NOTE: Bridge nodes do not automatically add the bootstrappers as trusted peers.
// NOTE: Use `MustNewNodeWithStore` to avoid this automatic configuration.
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
