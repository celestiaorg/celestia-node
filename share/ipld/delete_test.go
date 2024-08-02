package ipld

import (
	"context"
	"github.com/ipfs/boxo/exchange"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestDeleteNode_FullSquare(t *testing.T) {
	const size = 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bServ := NewMemBlockservice()

	shares := sharetest.RandShares(t, size*size)
	eds, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	keys, err := bServ.Blockstore().AllKeysChan(ctx)
	require.NoError(t, err)

	var preDeleteCount int
	for range keys {
		preDeleteCount++
	}
	require.NotZero(t, preDeleteCount)

	rowRoots, err := eds.RowRoots()
	require.NoError(t, err)
	for _, root := range rowRoots {
		err := DeleteNode(ctx, bServ, MustCidFromNamespacedSha256(root))
		require.NoError(t, err)
	}
	colRoots, err := eds.ColRoots()
	require.NoError(t, err)
	for _, root := range colRoots {
		err := DeleteNode(ctx, bServ, MustCidFromNamespacedSha256(root))
		require.NoError(t, err)
	}

	keys, err = bServ.Blockstore().AllKeysChan(ctx)
	require.NoError(t, err)

	var postDeleteCount int
	for range keys {
		postDeleteCount++
	}
	require.Zero(t, postDeleteCount)
}

func TestDeleteNode_Sample(t *testing.T) {
	const size = 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	full := NewMemBlockservice()

	shares := sharetest.RandShares(t, size*size)
	eds, err := AddShares(ctx, shares, full)
	require.NoError(t, err)

	rowRoots, err := eds.RowRoots()
	require.NoError(t, err)

	bstore := blockstore.NewBlockstore(sync.MutexWrap(datastore.NewMapDatastore()))
	exch := &fakeSessionExchange{
		Interface: offline.Exchange(full.Blockstore()),
		session:   offline.Exchange(full.Blockstore()),
	}
	light := NewBlockservice(bstore, exch)

	cid := MustCidFromNamespacedSha256(rowRoots[0])
	_, err = GetShare(ctx, light, cid, 0, len(rowRoots))
	require.NoError(t, err)

	keys, err := light.Blockstore().AllKeysChan(ctx)
	require.NoError(t, err)

	var preDeleteCount int
	for range keys {
		preDeleteCount++
	}
	require.NotZero(t, preDeleteCount)

	for _, root := range rowRoots {
		err := DeleteNode(ctx, light, MustCidFromNamespacedSha256(root))
		require.NoError(t, err)
	}

	keys, err = light.Blockstore().AllKeysChan(ctx)
	require.NoError(t, err)

	var postDeleteCount int
	for range keys {
		postDeleteCount++
	}
	require.Zero(t, postDeleteCount)
}

var _ exchange.SessionExchange = (*fakeSessionExchange)(nil)

type fakeSessionExchange struct {
	exchange.Interface
	session exchange.Fetcher
}

func (fe *fakeSessionExchange) NewSession(ctx context.Context) exchange.Fetcher {
	if ctx == nil {
		panic("nil context")
	}
	return fe.session
}
