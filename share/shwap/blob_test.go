package shwap

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v6/pkg/wrapper"
	"github.com/celestiaorg/go-square/merkle"
	"github.com/celestiaorg/go-square/v3/inclusion"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
)

func Test_GetBlob(t *testing.T) {
	var (
		blobSize0 = 32
		blobSize1 = 32
	)

	libBlobs, err := libshare.GenerateV0Blobs([]int{blobSize0, blobSize1}, true)
	require.NoError(t, err)

	sort.Slice(libBlobs, func(i, j int) bool {
		val := bytes.Compare(libBlobs[i].Namespace().ID(), libBlobs[j].Namespace().ID())
		return val < 0
	})

	shrs := make([]libshare.Share, 0, 64)
	for _, blob := range libBlobs {
		shares, err := blob.ToShares()
		require.NoError(t, err)
		shrs = append(shrs, shares...)
	}

	com1, err := inclusion.CreateCommitment(libBlobs[0], merkle.HashFromByteSlices, subtreeRootThreshold)
	require.NoError(t, err)

	odsSize := int(utils.SquareSize(len(shrs)))
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(shrs),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)))
	require.NoError(t, err)

	roots, err := eds.RowRoots()
	require.NoError(t, err)

	extendedRowShares := make([][]libshare.Share, 8)

	for index := 0; index < odsSize; index++ {
		rawShare := eds.Row(uint(index))
		sh, err := libshare.FromBytes(rawShare)
		require.NoError(t, err)
		extendedRowShares[index] = sh
	}

	blobContainer, err := BlobFromShares(extendedRowShares, shrs[0].Namespace(), com1, 8)
	require.NoError(t, err)
	err = blobContainer.Verify(roots, com1)
	require.NoError(t, err)

	result, err := BlobsFromShares(extendedRowShares, shrs[0].Namespace(), 8)
	require.NoError(t, err)
	assert.Equal(t, 2, len(result))
}
