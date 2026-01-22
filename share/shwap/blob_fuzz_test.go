package shwap

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v6/pkg/wrapper"
	"github.com/celestiaorg/go-square/merkle"
	"github.com/celestiaorg/go-square/v3/inclusion"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
)

func FuzzBlobFromShares(f *testing.F) {
	if testing.Short() {
		f.Skip("in -short mode")
	}

	// Add seed corpus with various blob sizes
	f.Add(6, 257)
	f.Add(32, 32)
	f.Add(64, 64)
	f.Add(128, 256)
	f.Add(16, 512)
	f.Add(256, 16)

	f.Fuzz(func(t *testing.T, blobSize0, blobSize1 int) {
		// Constrain blob sizes to reasonable values
		if blobSize0 <= 0 || blobSize0 > 1024 {
			return
		}
		if blobSize1 <= 0 || blobSize1 > 1024 {
			return
		}

		libBlobs, err := libshare.GenerateV0Blobs([]int{blobSize0, blobSize1}, true)
		if err != nil {
			return
		}

		// Sort blobs by namespace for consistent ordering
		sort.Slice(libBlobs, func(i, j int) bool {
			return bytes.Compare(libBlobs[i].Namespace().ID(), libBlobs[j].Namespace().ID()) < 0
		})

		shrs := make([]libshare.Share, 0)
		for _, blob := range libBlobs {
			shares, err := blob.ToShares()
			if err != nil {
				return
			}
			shrs = append(shrs, shares...)
		}

		if len(shrs) == 0 {
			return
		}

		odsSize := int(utils.SquareSize(len(shrs)))
		eds, err := rsmt2d.ComputeExtendedDataSquare(
			libshare.ToBytes(shrs),
			share.DefaultRSMT2DCodec(),
			wrapper.NewConstructor(uint64(odsSize)))
		if err != nil {
			return
		}

		roots, err := eds.RowRoots()
		if err != nil {
			return
		}

		extendedRowShares := make([][]libshare.Share, odsSize)
		for index := 0; index < odsSize; index++ {
			rawShare := eds.Row(uint(index))
			sh, err := libshare.FromBytes(rawShare)
			if err != nil {
				return
			}
			extendedRowShares[index] = sh
		}

		// Test BlobFromShares with first blob
		com0, err := inclusion.CreateCommitment(libBlobs[0], merkle.HashFromByteSlices, subtreeRootThreshold)
		if err != nil {
			return
		}

		blobContainer, err := BlobFromShares(extendedRowShares, libBlobs[0].Namespace(), com0, odsSize)
		require.NoError(t, err)
		require.NotNil(t, blobContainer)

		err = blobContainer.Verify(roots, com0)
		require.NoError(t, err)

		// Verify the reconstructed blob matches the original
		reconstructed, err := blobContainer.Blob()
		require.NoError(t, err)
		require.Equal(t, libBlobs[0].Data(), reconstructed.Data())
	})
}

func FuzzBlobsFromShares(f *testing.F) {
	if testing.Short() {
		f.Skip("in -short mode")
	}

	// Add seed corpus with various blob sizes
	f.Add(32, 32)
	f.Add(64, 64)
	f.Add(128, 256)
	f.Add(16, 512)
	f.Add(256, 16)

	f.Fuzz(func(t *testing.T, blobSize0, blobSize1 int) {
		// Constrain blob sizes to reasonable values
		if blobSize0 <= 0 || blobSize0 > 1024 {
			return
		}
		if blobSize1 <= 0 || blobSize1 > 1024 {
			return
		}

		libBlobs, err := libshare.GenerateV0Blobs([]int{blobSize0, blobSize1}, true)
		if err != nil {
			return
		}

		// Sort blobs by namespace
		sort.Slice(libBlobs, func(i, j int) bool {
			return bytes.Compare(libBlobs[i].Namespace().ID(), libBlobs[j].Namespace().ID()) < 0
		})

		shrs := make([]libshare.Share, 0)
		for _, blob := range libBlobs {
			shares, err := blob.ToShares()
			if err != nil {
				return
			}
			shrs = append(shrs, shares...)
		}

		if len(shrs) == 0 {
			return
		}

		odsSize := int(utils.SquareSize(len(shrs)))
		eds, err := rsmt2d.ComputeExtendedDataSquare(
			libshare.ToBytes(shrs),
			share.DefaultRSMT2DCodec(),
			wrapper.NewConstructor(uint64(odsSize)))
		if err != nil {
			return
		}

		roots, err := eds.RowRoots()
		if err != nil {
			return
		}

		extendedRowShares := make([][]libshare.Share, odsSize)
		for index := 0; index < odsSize; index++ {
			rawShare := eds.Row(uint(index))
			sh, err := libshare.FromBytes(rawShare)
			if err != nil {
				return
			}
			extendedRowShares[index] = sh
		}

		// Test BlobsFromShares - get all blobs in the namespace
		result, err := BlobsFromShares(extendedRowShares, libBlobs[0].Namespace(), odsSize)
		require.NoError(t, err)
		require.NotEmpty(t, result)

		// Verify each blob
		for _, blob := range result {
			err = blob.VerifyInclusion(roots)
			require.NoError(t, err)
		}
	})
}

func FuzzBlobFromSharesWithSameNamespace(f *testing.F) {
	if testing.Short() {
		f.Skip("in -short mode")
	}

	// Add seed corpus
	f.Add(32, 32, 32)
	f.Add(64, 128, 64)
	f.Add(16, 16, 16)

	f.Fuzz(func(t *testing.T, blobSize0, blobSize1, blobSize2 int) {
		// Constrain blob sizes
		if blobSize0 <= 0 || blobSize0 > 512 {
			return
		}
		if blobSize1 <= 0 || blobSize1 > 512 {
			return
		}
		if blobSize2 <= 0 || blobSize2 > 512 {
			return
		}

		// Generate blobs with the same namespace
		libBlobs, err := libshare.GenerateV0Blobs([]int{blobSize0, blobSize1, blobSize2}, false)
		if err != nil {
			return
		}

		shrs := make([]libshare.Share, 0)
		for _, blob := range libBlobs {
			shares, err := blob.ToShares()
			if err != nil {
				return
			}
			shrs = append(shrs, shares...)
		}

		if len(shrs) == 0 {
			return
		}

		odsSize := int(utils.SquareSize(len(shrs)))
		eds, err := rsmt2d.ComputeExtendedDataSquare(
			libshare.ToBytes(shrs),
			share.DefaultRSMT2DCodec(),
			wrapper.NewConstructor(uint64(odsSize)))
		if err != nil {
			return
		}

		roots, err := eds.RowRoots()
		if err != nil {
			return
		}

		extendedRowShares := make([][]libshare.Share, odsSize)
		for index := 0; index < odsSize; index++ {
			rawShare := eds.Row(uint(index))
			sh, err := libshare.FromBytes(rawShare)
			if err != nil {
				return
			}
			extendedRowShares[index] = sh
		}

		// All blobs should have the same namespace
		ns := libBlobs[0].Namespace()

		// Test BlobsFromShares returns all blobs
		result, err := BlobsFromShares(extendedRowShares, ns, odsSize)
		require.NoError(t, err)
		require.Equal(t, len(libBlobs), len(result))

		// Test BlobFromShares can find each blob by commitment
		for i, libBlob := range libBlobs {
			com, err := inclusion.CreateCommitment(libBlob, merkle.HashFromByteSlices, subtreeRootThreshold)
			require.NoError(t, err)

			blobContainer, err := BlobFromShares(extendedRowShares, ns, com, odsSize)
			require.NoError(t, err, "failed to find blob %d", i)
			require.NotNil(t, blobContainer)

			err = blobContainer.Verify(roots, com)
			require.NoError(t, err)

			// Verify data matches
			reconstructed, err := blobContainer.Blob()
			require.NoError(t, err)
			require.Equal(t, libBlob.Data(), reconstructed.Data())
		}
	})
}
