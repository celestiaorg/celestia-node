package ipldv2

import (
	"context"
	"hash"
	"testing"
	"time"

	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestV2Roundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	store := headertest.NewStore(t)

	mh.Register(multihashCode, func() hash.Hash {
		return &Hasher{get: store}
	})

	height := uint64(1)
	shrIdx := 5

	dn := availability_test.NewTestDAGNet(ctx, t)
	srv1 := dn.NewTestNode().BlockService
	srv2 := dn.NewTestNode().BlockService
	dn.ConnectAll()

	square := edstest.RandEDS(t, 16)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	hdr, err := store.GetByHeight(ctx, height)
	require.NoError(t, err)
	hdr.DAH = root
	hdr.DataHash = root.Hash()

	file, err := eds.CreateFile(t.TempDir()+"/eds_file", square)
	require.NoError(t, err)

	shr, prf, err := file.ShareWithProof(shrIdx, rsmt2d.Row)
	require.NoError(t, err)

	nd, err := MakeNode(5, shr, prf, hdr.DataHash, hdr.Height())
	require.NoError(t, err)

	err = srv1.AddBlock(ctx, nd)
	require.NoError(t, err)

	cid, err := ShareCID(root.Hash(), hdr.Height(), shrIdx)
	require.NoError(t, err)
	require.True(t, cid.Equals(nd.Cid()))

	b, err := srv2.GetBlock(ctx, cid)
	require.NoError(t, err)

	assert.EqualValues(t, b.RawData(), nd.RawData())
}
