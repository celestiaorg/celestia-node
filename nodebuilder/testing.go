package nodebuilder

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"

	"github.com/celestiaorg/celestia-node/core"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	store := MockStore(t, cfg)
	_, _, coreCfg := core.StartTestKVApp(ctx, t)
	endpoint, err := core.GetEndpoint(coreCfg)
	require.NoError(t, err)
	ip, port, err := net.SplitHostPort(endpoint)
	require.NoError(t, err)
	cfg.Core.IP = ip
	cfg.Core.RPCPort = port
	cfg.RPC.Port = "26655"

	opts = append(opts,
		state.WithKeyringSigner(TestKeyringSigner(t)),
	)
	nd, err := New(tp, p2p.Private, store, opts...)
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
