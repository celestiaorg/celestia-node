package node

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"

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
	node, _ := core.StartTestKVApp(t)
	opts = append(
		opts,
		WithRemoteCore(core.GetEndpoint(node)),
		WithNetwork(params.Private),
		WithCustomKeyringSigner(InMemKeyringSigner(t)),
	)
	store := MockStore(t, DefaultConfig(tp))
	nd, err := New(tp, store, opts...)
	require.NoError(t, err)
	return nd
}

func InMemKeyringSigner(t *testing.T) *apptypes.KeyringSigner {
	ring := keyring.NewInMemory()
	signer := apptypes.NewKeyringSigner(ring, "", string(params.Private))
	_, _, err := signer.NewMnemonic("test_celes", keyring.English, "",
		"", hd.Secp256k1)
	require.NoError(t, err)
	return signer
}
