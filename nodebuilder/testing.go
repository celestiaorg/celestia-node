package nodebuilder

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header/mocks"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// MockStore provides mock in memory Store for testing purposes.
func MockStore(t *testing.T, cfg *Config) Store {
	t.Helper()
	store := NewMemStore()
	err := store.PutConfig(cfg)
	require.NoError(t, err)
	return store
}

func TestNode(t *testing.T, tp node.Type, opts ...fx.Option) *Node {
	return TestNodeWithConfig(t, tp, DefaultConfig(tp), opts...)
}

func TestNodeWithConfig(t *testing.T, tp node.Type, cfg *Config, opts ...fx.Option) *Node {
	// avoids port conflicts
	cfg.RPC.Port = "0"
	opts = append(opts,
		// avoid writing keyring on disk
		state.WithKeyringSigner(TestKeyringSigner(t)),
		// temp dir for the eds store FIXME: Should be in mem
		fx.Replace(node.StorePath(t.TempDir())),
		// avoid requesting trustedPeer during initialization
		fxutil.ReplaceAs(mocks.NewStore(t, 20), new(header.InitStore)),
	)

	// in fact, we don't need core.Client in tests, but Bridge requires is a valid one
	// or fails otherwise with failed attempt to connect with custom build client
	if tp == node.Bridge {
		cctx := core.StartTestNode(t)
		opts = append(opts,
			fxutil.ReplaceAs(cctx.Client, new(core.Client)),
		)
	}

	nd, err := New(tp, p2p.Private, MockStore(t, cfg), opts...)
	require.NoError(t, err)
	return nd
}

func TestKeyringSigner(t *testing.T) *apptypes.KeyringSigner {
	encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	ring := keyring.NewInMemory(encConf.Codec)
	signer := apptypes.NewKeyringSigner(ring, "", string(p2p.Private))
	_, _, err := signer.NewMnemonic("test_celes", keyring.English, "",
		"", hd.Secp256k1)
	require.NoError(t, err)
	return signer
}
