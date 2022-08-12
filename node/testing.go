package node

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/node/config"
	"github.com/celestiaorg/celestia-node/params"
)

// MockStore provides mock in memory Store for testing purposes.
func MockStore(t *testing.T, cfg *config.Config) Store {
	t.Helper()
	store := NewMemStore()
	err := store.PutConfig(cfg)
	require.NoError(t, err)
	return store
}

func TestNode(t *testing.T, tp config.NodeType, opts ...config.Option) *Node {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	store := MockStore(t, config.DefaultConfig(tp))
	_, _, cfg := core.StartTestKVApp(ctx, t)
	endpoint, err := core.GetEndpoint(cfg)
	require.NoError(t, err)
	ip, port, err := net.SplitHostPort(endpoint)
	require.NoError(t, err)
	opts = append(opts,
		config.WithRemoteCoreIP(ip),
		config.WithRemoteCorePort(port),
		config.WithNetwork(params.Private),
		config.WithRPCPort("0"),
		config.WithKeyringSigner(TestKeyringSigner(t)),
	)
	nd, err := New(tp, store, opts...)
	require.NoError(t, err)
	return nd
}

func TestKeyringSigner(t *testing.T) *apptypes.KeyringSigner {
	encConf := encoding.MakeEncodingConfig(app.ModuleEncodingRegisters...)
	ring := keyring.NewInMemory(encConf.Codec)
	signer := apptypes.NewKeyringSigner(ring, "", string(params.Private))
	_, _, err := signer.NewMnemonic("test_celes", keyring.English, "",
		"", hd.Secp256k1)
	require.NoError(t, err)
	return signer
}
