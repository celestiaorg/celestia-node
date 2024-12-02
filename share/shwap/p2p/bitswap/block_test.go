package bitswap

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

const (
	testCodec         = 0x9999
	testMultihashCode = 0x9999
	testBlockSize     = 256
	testIDSize        = 2
)

func init() {
	registerBlock(
		testMultihashCode,
		testCodec,
		testBlockSize,
		testIDSize,
		func(cid cid.Cid) (Block, error) {
			return newEmptyTestBlock(cid)
		},
	)
}

func testBlockstore(ctx context.Context, t *testing.T, items int) (blockstore.Blockstore, *cid.Set) {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))

	cids := cid.NewSet()
	for i := range items {
		blk := newTestBlock(i)
		bitswapBlk, err := convertBitswap(blk)
		require.NoError(t, err)
		err = bstore.Put(ctx, bitswapBlk)
		require.NoError(t, err)
		cids.Add(blk.CID())
	}
	return bstore, cids
}

type testID uint16

func (t testID) MarshalBinary() (data []byte, err error) {
	data = binary.BigEndian.AppendUint16(data, uint16(t))
	return data, nil
}

func (t *testID) UnmarshalBinary(data []byte) error {
	*t = testID(binary.BigEndian.Uint16(data))
	return nil
}

type testBlock struct {
	id   testID
	data []byte
}

func newTestBlock(id int) *testBlock {
	bytes := make([]byte, testBlockSize)
	_, _ = crand.Read(bytes)
	return &testBlock{id: testID(id), data: bytes}
}

func newEmptyTestBlock(cid cid.Cid) (*testBlock, error) {
	idData, err := extractFromCID(cid)
	if err != nil {
		return nil, err
	}

	var id testID
	err = id.UnmarshalBinary(idData)
	if err != nil {
		return nil, err
	}

	return &testBlock{id: id}, nil
}

func (t *testBlock) CID() cid.Cid {
	return encodeToCID(t.id, testMultihashCode, testCodec)
}

func (t *testBlock) Height() uint64 {
	return 1
}

func (t *testBlock) Populate(context.Context, eds.Accessor) error {
	return nil // noop
}

func (t *testBlock) Marshal() ([]byte, error) {
	return t.data, nil
}

func (t *testBlock) UnmarshalFn(*share.AxisRoots) UnmarshalFn {
	return func(bytes, _ []byte) error {
		t.data = bytes
		time.Sleep(time.Millisecond * 1)
		return nil
	}
}
