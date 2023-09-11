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

	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestV2Roundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	mh.Register(multihashCode, func() hash.Hash {
		return &Hasher{}
	})

	shrIdx := 5

	dn := availability_test.NewTestDAGNet(ctx, t)
	srv1 := dn.NewTestNode().BlockService
	srv2 := dn.NewTestNode().BlockService
	dn.ConnectAll()

	square := edstest.RandEDS(t, 16)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	file, err := eds.CreateFile(t.TempDir()+"/eds_file", square)
	require.NoError(t, err)

	shr, prf, err := file.ShareWithProof(shrIdx, rsmt2d.Row)
	require.NoError(t, err)

	nd, err := MakeNode(root, rsmt2d.Row, shrIdx, shr, prf)
	require.NoError(t, err)

	err = srv1.AddBlock(ctx, nd)
	require.NoError(t, err)

	cid, err := ShareCID(root, shrIdx, rsmt2d.Row)
	require.NoError(t, err)
	require.True(t, cid.Equals(nd.Cid()))

	b, err := srv2.GetBlock(ctx, cid)
	require.NoError(t, err)

	assert.EqualValues(t, b.RawData(), nd.RawData())
}
