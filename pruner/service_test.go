package pruner

import (
	"context"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	hdr "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

/*
	| toPrune  | availability window |
*/

// TODO @renaynay: document
var (
	availWindow = AvailabilityWindow(time.Millisecond)
	blockTime   = time.Millisecond * 100
	gcCycle     = time.Millisecond * 500
)

func TestService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := headertest.NewStore(t)

	mp := &mockPruner{}

	serv := NewService(
		mp,
		availWindow,
		store,
		sync.MutexWrap(datastore.NewMapDatastore()),
		blockTime,
		WithGCCycle(gcCycle),
	)

	gen, err := store.GetByHeight(ctx, 1)
	require.NoError(t, err)

	err = serv.updateCheckpoint(ctx, gen)
	require.NoError(t, err)

	err = serv.Start(ctx)
	require.NoError(t, err)

	time.Sleep(time.Second)

	err = serv.Stop(ctx)
	require.NoError(t, err)

	t.Log(len(mp.deletedHeaderHashes))
}

type mockPruner struct {
	deletedHeaderHashes []hdr.Hash
}

func (mp *mockPruner) Prune(_ context.Context, headers ...*header.ExtendedHeader) error {
	for _, h := range headers {
		mp.deletedHeaderHashes = append(mp.deletedHeaderHashes, h.Hash())
	}
	return nil
}
