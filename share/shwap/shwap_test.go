package shwap

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/routing/offline"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	record "github.com/libp2p/go-libp2p-record"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

// TestSampleRoundtripGetBlock tests full protocol round trip of:
// EDS -> Sample -> IPLDBlock -> BlockService -> Bitswap and in reverse.
func TestSampleRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	b := NewTestBlockstore(t)
	eds := edstest.RandEDS(t, 8)
	height := b.AddEds(eds)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	client := remoteClient(ctx, t, b)

	width := int(eds.Width())
	for i := 0; i < width*width; i++ {
		smpl, err := NewSampleFromEDS(RowProofType, i, eds, height) // TODO: Col
		require.NoError(t, err)

		sampleVerifiers.Add(smpl.SampleID, func(sample Sample) error {
			return sample.Verify(root)
		})

		cid := smpl.Cid()
		blkOut, err := client.GetBlock(ctx, cid)
		require.NoError(t, err)
		require.EqualValues(t, cid, blkOut.Cid())

		smpl, err = SampleFromBlock(blkOut)
		require.NoError(t, err)

		err = smpl.Verify(root)
		require.NoError(t, err)
	}
}

// TODO: Debug why is it flaky
func TestSampleRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	b := NewTestBlockstore(t)
	eds := edstest.RandEDS(t, 8)
	height := b.AddEds(eds)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := remoteClient(ctx, t, b)

	set := cid.NewSet()
	width := int(eds.Width())
	for i := 0; i < width*width; i++ {
		smpl, err := NewSampleFromEDS(RowProofType, i, eds, height) // TODO: Col
		require.NoError(t, err)
		set.Add(smpl.Cid())

		sampleVerifiers.Add(smpl.SampleID, func(sample Sample) error {
			return sample.Verify(root)
		})
	}

	blks, err := client.GetBlocks(ctx, set.Keys())
	require.NoError(t, err)

	err = set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			require.True(t, set.Has(blk.Cid()))

			smpl, err := SampleFromBlock(blk)
			require.NoError(t, err)

			err = smpl.Verify(root) // bitswap already performed validation and this is only for testing
			require.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	require.NoError(t, err)
}

func TestRowRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	b := NewTestBlockstore(t)
	eds := edstest.RandEDS(t, 8)
	height := b.AddEds(eds)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := remoteClient(ctx, t, b)

	width := int(eds.Width())
	for i := 0; i < width; i++ {
		row, err := NewRowFromEDS(height, i, eds)
		require.NoError(t, err)

		rowVerifiers.Add(row.RowID, func(row Row) error {
			return row.Verify(root)
		})

		cid := row.Cid()
		blkOut, err := client.GetBlock(ctx, cid)
		require.NoError(t, err)
		require.EqualValues(t, cid, blkOut.Cid())

		row, err = RowFromBlock(blkOut)
		require.NoError(t, err)

		err = row.Verify(root)
		require.NoError(t, err)
	}
}

func TestRowRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	b := NewTestBlockstore(t)
	eds := edstest.RandEDS(t, 8)
	height := b.AddEds(eds)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := remoteClient(ctx, t, b)

	set := cid.NewSet()
	width := int(eds.Width())
	for i := 0; i < width; i++ {
		row, err := NewRowFromEDS(height, i, eds)
		require.NoError(t, err)
		set.Add(row.Cid())

		rowVerifiers.Add(row.RowID, func(row Row) error {
			return row.Verify(root)
		})
	}

	blks, err := client.GetBlocks(ctx, set.Keys())
	require.NoError(t, err)

	err = set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			require.True(t, set.Has(blk.Cid()))

			row, err := RowFromBlock(blk)
			require.NoError(t, err)

			err = row.Verify(root)
			require.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	require.NoError(t, err)
}

func TestDataRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	b := NewTestBlockstore(t)
	namespace := sharetest.RandV0Namespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 16)
	height := b.AddEds(eds)
	client := remoteClient(ctx, t, b)

	nds, err := NewDataFromEDS(eds, height, namespace)
	require.NoError(t, err)

	for _, nd := range nds {
		dataVerifiers.Add(nd.DataID, func(data Data) error {
			return data.Verify(root)
		})

		cid := nd.Cid()
		blkOut, err := client.GetBlock(ctx, cid)
		require.NoError(t, err)
		require.EqualValues(t, cid, blkOut.Cid())

		ndOut, err := DataFromBlock(blkOut)
		require.NoError(t, err)

		err = ndOut.Verify(root)
		require.NoError(t, err)
	}
}

func TestDataRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	b := NewTestBlockstore(t)
	namespace := sharetest.RandV0Namespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 16)
	height := b.AddEds(eds)
	client := remoteClient(ctx, t, b)

	nds, err := NewDataFromEDS(eds, height, namespace)
	require.NoError(t, err)

	set := cid.NewSet()
	for _, nd := range nds {
		set.Add(nd.Cid())

		dataVerifiers.Add(nd.DataID, func(data Data) error {
			return data.Verify(root)
		})
	}

	blks, err := client.GetBlocks(ctx, set.Keys())
	require.NoError(t, err)

	err = set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			require.True(t, set.Has(blk.Cid()))

			smpl, err := DataFromBlock(blk)
			require.NoError(t, err)

			err = smpl.Verify(root)
			require.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	require.NoError(t, err)
}

func remoteClient(ctx context.Context, t *testing.T, bstore blockstore.Blockstore) exchange.Fetcher {
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})
	_ = bitswap.New(
		ctx,
		network.NewFromIpfsHost(net.Hosts()[0], routing),
		bstore,
	)

	dstoreClient := dssync.MutexWrap(ds.NewMapDatastore())
	bstoreClient := blockstore.NewBlockstore(dstoreClient)
	routingClient := offline.NewOfflineRouter(dstoreClient, record.NamespacedValidator{})

	bitswapClient := bitswap.New(
		ctx,
		network.NewFromIpfsHost(net.Hosts()[1], routingClient),
		bstoreClient,
	)

	err = net.ConnectAllButSelf()
	require.NoError(t, err)

	return bitswapClient
}
