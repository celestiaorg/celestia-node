package ipldv2

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/routing/offline"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	record "github.com/libp2p/go-libp2p-record"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

var axisTypes = []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row}

// TestSampleRoundtripGetBlock tests full protocol round trip of:
// EDS -> Sample -> IPLDBlock -> BlockService -> Bitswap and in reverse.
func TestSampleRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	sqr := edstest.RandEDS(t, 8)
	b := edsBlockstore(sqr)
	client := remoteClient(ctx, t, b)

	width := int(sqr.Width())
	for _, axisType := range axisTypes {
		for i := 0; i < width*width; i++ {
			smpl, err := NewSampleFromEDS(axisType, i, sqr, 1)
			require.NoError(t, err)

			cid, err := smpl.SampleID.Cid()
			require.NoError(t, err)

			blkOut, err := client.GetBlock(ctx, cid)
			require.NoError(t, err)
			assert.EqualValues(t, cid, blkOut.Cid())

			smpl, err = SampleFromBlock(blkOut)
			assert.NoError(t, err)

			err = smpl.Validate() // bitswap already performed validation and this is only for testing
			assert.NoError(t, err)
		}
	}
}

func TestSampleRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*100)
	defer cancel()

	sqr := edstest.RandEDS(t, 8)
	b := edsBlockstore(sqr)
	client := remoteClient(ctx, t, b)

	set := cid.NewSet()
	width := int(sqr.Width())
	for _, axisType := range axisTypes {
		for i := 0; i < width*width; i++ {
			smpl, err := NewSampleFromEDS(axisType, i, sqr, 1)
			require.NoError(t, err)

			cid, err := smpl.SampleID.Cid()
			require.NoError(t, err)

			set.Add(cid)
		}
	}

	blks := client.GetBlocks(ctx, set.Keys())
	err := set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			assert.True(t, set.Has(blk.Cid()))

			smpl, err := SampleFromBlock(blk)
			assert.NoError(t, err)

			err = smpl.Validate() // bitswap already performed validation and this is only for testing
			assert.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	assert.NoError(t, err)
}

func TestAxisRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10000)
	defer cancel()

	sqr := edstest.RandEDS(t, 16)
	b := edsBlockstore(sqr)
	client := remoteClient(ctx, t, b)

	width := int(sqr.Width())
	for _, axisType := range axisTypes {
		for i := 0; i < width; i++ {
			smpl, err := NewAxisFromEDS(axisType, i, sqr, 1)
			require.NoError(t, err)

			cid, err := smpl.AxisID.Cid()
			require.NoError(t, err)

			blkOut, err := client.GetBlock(ctx, cid)
			require.NoError(t, err)
			assert.EqualValues(t, cid, blkOut.Cid())

			smpl, err = AxisFromBlock(blkOut)
			assert.NoError(t, err)

			err = smpl.Validate() // bitswap already performed validation and this is only for testing
			assert.NoError(t, err)
		}
	}
}

func TestAxisRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	sqr := edstest.RandEDS(t, 16)
	b := edsBlockstore(sqr)
	client := remoteClient(ctx, t, b)

	set := cid.NewSet()
	width := int(sqr.Width())
	for _, axisType := range axisTypes {
		for i := 0; i < width; i++ {
			smpl, err := NewAxisFromEDS(axisType, i, sqr, 1)
			require.NoError(t, err)

			cid, err := smpl.AxisID.Cid()
			require.NoError(t, err)

			set.Add(cid)
		}
	}

	blks := client.GetBlocks(ctx, set.Keys())
	err := set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			assert.True(t, set.Has(blk.Cid()))

			smpl, err := AxisFromBlock(blk)
			assert.NoError(t, err)

			err = smpl.Validate() // bitswap already performed validation and this is only for testing
			assert.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	assert.NoError(t, err)
}

func TestDataRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	namespace := sharetest.RandV0Namespace()
	sqr, _ := edstest.RandEDSWithNamespace(t, namespace, 16)
	b := edsBlockstore(sqr)
	client := remoteClient(ctx, t, b)

	nds, err := NewDataFromEDS(sqr, 1, namespace)
	require.NoError(t, err)

	for _, nd := range nds {
		cid, err := nd.DataID.Cid()
		require.NoError(t, err)

		blkOut, err := client.GetBlock(ctx, cid)
		require.NoError(t, err)
		assert.EqualValues(t, cid, blkOut.Cid())

		ndOut, err := DataFromBlock(blkOut)
		assert.NoError(t, err)

		err = ndOut.Validate() // bitswap already performed validation and this is only for testing
		assert.NoError(t, err)
	}
}

func TestDataRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	namespace := sharetest.RandV0Namespace()
	sqr, _ := edstest.RandEDSWithNamespace(t, namespace, 16)
	b := edsBlockstore(sqr)
	client := remoteClient(ctx, t, b)

	nds, err := NewDataFromEDS(sqr, 1, namespace)
	require.NoError(t, err)

	set := cid.NewSet()
	for _, nd := range nds {
		cid, err := nd.DataID.Cid()
		require.NoError(t, err)
		set.Add(cid)
	}

	blks := client.GetBlocks(ctx, set.Keys())
	err = set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			assert.True(t, set.Has(blk.Cid()))

			smpl, err := DataFromBlock(blk)
			assert.NoError(t, err)

			err = smpl.Validate() // bitswap already performed validation and this is only for testing
			assert.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	assert.NoError(t, err)
}

func remoteClient(ctx context.Context, t *testing.T, bstore blockstore.Blockstore) blockservice.BlockService {
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

	return NewBlockService(bstoreClient, bitswapClient)
}
