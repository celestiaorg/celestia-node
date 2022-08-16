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
	"github.com/celestiaorg/celestia-node/params"
)

// MockStore provides mock in memory Store for testing purposes.
func MockStore(t *testing.T, cfg *Config) Store {
	t.Helper()
	store := NewMemStore()
	err := store.PutConfig(cfg)
	require.NoError(t, err)
	return store
}

func TestNode(t *testing.T, tp Type, opts ...Option) *Node {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	store := MockStore(t, DefaultConfig(tp))
	_, _, cfg := core.StartTestKVApp(ctx, t)
	endpoint, err := core.GetEndpoint(cfg)
	require.NoError(t, err)
	ip, port, err := net.SplitHostPort(endpoint)
	require.NoError(t, err)
	opts = append(opts,
		WithRemoteCoreIP(ip),
		WithRemoteCorePort(port),
		WithNetwork(params.Private),
		WithRPCPort("0"),
		WithKeyringSigner(TestKeyringSigner(t)),
	)
	nd, err := New(tp, store, opts...)
	require.NoError(t, err)
	return nd
}

func TestKeyringSigner(t *testing.T) *apptypes.KeyringSigner {
	encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	ring := keyring.NewInMemory(encConf.Codec)
	signer := apptypes.NewKeyringSigner(ring, "", string(params.Private))
	_, _, err := signer.NewMnemonic("test_celes", keyring.English, "",
		"", hd.Secp256k1)
	require.NoError(t, err)
	return signer
}
