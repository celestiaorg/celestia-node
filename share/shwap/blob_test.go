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

// TestBlobRetrieval tests BlobFromShares and BlobsFromShares with a large ODS (128x128)
// containing many blobs across multiple namespaces. It verifies:
// - Correct blob retrieval by commitment (BlobFromShares)
// - Correct batch retrieval by namespace (BlobsFromShares)
// - Proof verification for all retrieved blobs
func TestBlobRetrieval(t *testing.T) {
	const odsSize = 128
	const numNamespaces = 8
	const blobsPerNamespace = 5

	namespaces := make([]libshare.Namespace, numNamespaces)
	for i := 0; i < numNamespaces; i++ {
		nsBytes := bytes.Repeat([]byte{byte(i + 1)}, libshare.NamespaceVersionZeroIDSize)
		ns, err := libshare.NewV0Namespace(nsBytes)
		require.NoError(t, err)
		namespaces[i] = ns
	}

	blobsByNs := make(map[string][]*libshare.Blob)
	var allBlobs []*libshare.Blob

	// Blob sizes in number of shares
	blobShareCounts := []int{
		1,
		2,
		5,
		13,
		64,
		129,
		200,
	}

	blobIdx := 0
	for _, ns := range namespaces {
		for i := 0; i < blobsPerNamespace; i++ {
			shareCount := blobShareCounts[blobIdx%len(blobShareCounts)]
			blobIdx++

			// Calculate data size to fill exactly shareCount shares
			var dataSize int
			if shareCount == 1 {
				dataSize = libshare.FirstSparseShareContentSize
			} else {
				dataSize = libshare.FirstSparseShareContentSize + (shareCount-1)*libshare.ContinuationSparseShareContentSize
			}

			// Use unique data pattern for each blob
			data := bytes.Repeat([]byte{byte(blobIdx), byte(shareCount)}, dataSize)
			blob, err := libshare.NewV0Blob(ns, data)
			require.NoError(t, err)

			nsKey := ns.String()
			blobsByNs[nsKey] = append(blobsByNs[nsKey], blob)
			allBlobs = append(allBlobs, blob)
		}
	}

	// Sort blobs by namespace (required for proper square layout)
	sort.Slice(allBlobs, func(i, j int) bool {
		return bytes.Compare(allBlobs[i].Namespace().Bytes(), allBlobs[j].Namespace().Bytes()) < 0
	})

	// Convert blobs to shares
	var allShares []libshare.Share
	// Track start index for each blob
	blobStartIndices := make(map[*libshare.Blob]int)
	for _, blob := range allBlobs {
		blobStartIndices[blob] = len(allShares)
		shares, err := blob.ToShares()
		require.NoError(t, err)
		allShares = append(allShares, shares...)
	}

	totalShares := odsSize * odsSize
	require.LessOrEqual(t, len(allShares), totalShares, "too many shares for ODS size")
	if len(allShares) < totalShares {
		padding := libshare.TailPaddingShares(totalShares - len(allShares))
		allShares = append(allShares, padding...)
	}

	// Build EDS
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(allShares),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)))
	require.NoError(t, err)

	roots, err := eds.RowRoots()
	require.NoError(t, err)

	// Extract extended row shares (includes parity data for proof generation)
	extendedRowShares := make([][]libshare.Share, odsSize)
	for i := 0; i < odsSize; i++ {
		rawShares := eds.Row(uint(i))
		sh, err := libshare.FromBytes(rawShares)
		require.NoError(t, err)
		extendedRowShares[i] = sh
	}

	// Test 1: BlobFromShares - retrieve each blob by commitment
	t.Run("BlobFromShares", func(t *testing.T) {
		for _, blob := range allBlobs {
			commitment, err := inclusion.CreateCommitment(blob, merkle.HashFromByteSlices, subtreeRootThreshold)
			require.NoError(t, err)

			retrieved, err := BlobFromShares(extendedRowShares, blob.Namespace(), commitment, odsSize)
			require.NoError(t, err, "failed to retrieve blob with namespace %x", blob.Namespace().Bytes())
			require.NotNil(t, retrieved)

			// Verify the blob data matches
			reconstructed, err := retrieved.Blob()
			require.NoError(t, err)
			require.Equal(t, blob.Data(), reconstructed.Data(), "blob data mismatch")

			// Verify the index
			expectedIdx := blobStartIndices[blob]
			require.Equal(t, expectedIdx, retrieved.Index(), "blob index mismatch")

			// Verify inclusion proof
			err = retrieved.Verify(roots, commitment)
			require.NoError(t, err, "proof verification failed for blob at index %d", expectedIdx)
		}
	})

	// Test 2: BlobsFromShares - retrieve all blobs for each namespace
	t.Run("BlobsFromShares", func(t *testing.T) {
		for _, ns := range namespaces {
			nsKey := ns.String()
			expectedBlobs := blobsByNs[nsKey]

			retrieved, err := BlobsFromShares(extendedRowShares, ns, odsSize)
			require.NoError(t, err)
			require.Len(t, retrieved, len(expectedBlobs))

			// Verify each retrieved blob
			for i, blob := range retrieved {
				// Verify data matches one of the expected blobs
				reconstructed, err := blob.Blob()
				require.NoError(t, err)

				// Find matching expected blob
				var matchingBlob *libshare.Blob
				for _, expected := range expectedBlobs {
					if bytes.Equal(expected.Data(), reconstructed.Data()) {
						matchingBlob = expected
						break
					}
				}
				require.NotNil(t, matchingBlob, "retrieved blob %d doesn't match any expected blob", i)

				// Verify commitment and proof
				commitment, err := inclusion.CreateCommitment(matchingBlob, merkle.HashFromByteSlices, subtreeRootThreshold)
				require.NoError(t, err)

				err = blob.Verify(roots, commitment)
				require.NoError(t, err, "proof verification failed for blob %d in namespace %x", i, ns.Bytes())
			}
		}
	})

	// Test 3: Verify mid-row blobs are handled correctly
	t.Run("MidRowBlobs", func(t *testing.T) {
		midRowCount := 0
		for _, blob := range allBlobs {
			startIdx := blobStartIndices[blob]
			col := startIdx % odsSize
			if col > 0 {
				midRowCount++

				commitment, err := inclusion.CreateCommitment(blob, merkle.HashFromByteSlices, subtreeRootThreshold)
				require.NoError(t, err)

				retrieved, err := BlobFromShares(extendedRowShares, blob.Namespace(), commitment, odsSize)
				require.NoError(t, err, "failed to retrieve mid-row blob at index %d (row=%d, col=%d)",
					startIdx, startIdx/odsSize, col)

				err = retrieved.Verify(roots, commitment)
				require.NoError(t, err, "proof verification failed for mid-row blob")
			}
		}
	})
}

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
