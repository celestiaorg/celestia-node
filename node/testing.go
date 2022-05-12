package node

import (
	"context"
	"testing"

	format "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/ipld"
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
	opts = append(opts, WithRemoteCore(core.GetEndpoint(node)), WithNetwork(params.Private))
	store := MockStore(t, DefaultConfig(tp))
	nd, err := New(tp, store, opts...)
	require.NoError(t, err)
	return nd
}

// WithFaultMaker is used to enable test cases with invalid blocks
func WithFaultMaker() Option {
	return func(sets *settings) {
		sets.opts = append(sets.opts, fx.Replace(faultMaker))
	}
}

func faultMaker(ctx context.Context,
	b *types.Block,
	comm *types.Commit,
	vals *types.ValidatorSet,
	dag format.NodeAdder) (*header.ExtendedHeader, error) {
	log.Info("using fault header maker...")
	var dah da.DataAvailabilityHeader

	namespacedShares, _ := b.Data.ComputeShares()

	extended, err := ipld.AddShares(ctx, namespacedShares.RawShares(), dag)
	if err != nil {
		return nil, err
	}

	if b.Height == int64(1) {
		t := testing.T{}
		size := len(namespacedShares.RawShares()) * 2
		extended = ipld.RandEDS(&t, size)
		shares := ipld.ExtractEDS(extended)
		copy(shares[0][consts.NamespaceSize:], shares[1][consts.NamespaceSize:])
		dah = ipld.CreateDAH(ctx, shares, uint64(extended.Width()), dag)
	}
	dah = da.NewDataAvailabilityHeader(extended)
	eh := &header.ExtendedHeader{
		RawHeader:    b.Header,
		DAH:          &dah,
		Commit:       comm,
		ValidatorSet: vals,
	}
	return eh, nil
}
