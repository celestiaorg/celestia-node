package nodebuilder

import (
	"net"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	stateModule "github.com/celestiaorg/celestia-node/nodebuilder/state"
)

const (
	TestKeyringName  = "test_celes"
	TestKeyringName1 = "test_celes1"
)

// MockStore provides mock in memory Store for testing purposes.
func MockStore(t *testing.T, cfg *Config) Store {
	t.Helper()
	store := NewMemStore()

	err := store.PutConfig(cfg)
	require.NoError(t, err)

	ks, err := store.Keystore()
	require.NoError(t, err)
	_, _, err = generateNewKey(ks.Keyring())
	require.NoError(t, err)

	return store
}

func TestNode(t *testing.T, tp node.Type, opts ...fx.Option) *Node {
	return TestNodeWithConfig(t, tp, DefaultConfig(tp), opts...)
}

func TestNodeWithConfig(t *testing.T, tp node.Type, cfg *Config, opts ...fx.Option) *Node {
	// avoids port conflicts
	cfg.RPC.Port = "0"
	cfg.Header.TrustedPeers = []string{"/ip4/1.2.3.4/tcp/12345/p2p/12D3KooWNaJ1y1Yio3fFJEXCZyd1Cat3jmrPdgkYCrHfKD3Ce21p"}

	// Bridge node requires a core endpoint. Set config IP/Port so the
	// production grpcClient provider creates a properly configured connection.
	if tp == node.Bridge && cfg.Core.IP == "" {
		tn := core.StartTestNode(t)
		var err error
		cfg.Core.IP, cfg.Core.Port, err = net.SplitHostPort(tn.GRPCClient.Target())
		require.NoError(t, err)
	}

	store := MockStore(t, cfg)
	ks, err := store.Keystore()
	require.NoError(t, err)
	kr := ks.Keyring()

	// create a key in the keystore to be used by the core accessor
	_, _, err = kr.NewMnemonic(TestKeyringName, keyring.English, "", "", hd.Secp256k1)
	require.NoError(t, err)
	cfg.State.DefaultKeyName = TestKeyringName
	_, accName, err := stateModule.Keyring(cfg.State, ks)
	require.NoError(t, err)
	require.Equal(t, TestKeyringName, string(accName))

	opts = append(opts,
		// temp dir for the eds store FIXME: Should be in mem
		fx.Replace(node.StorePath(t.TempDir())),
		// avoid requesting trustedPeer during initialization
		fxutil.ReplaceAs(headertest.NewStore(t), new(libhead.Store[*header.ExtendedHeader])),
	)

	nd, err := New(tp, p2p.Private, store, opts...)
	require.NoError(t, err)
	return nd
}
