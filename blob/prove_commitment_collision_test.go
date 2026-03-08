package blob

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v7/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
)

// TestProveCommitmentCollision reproduces a bug where ProveCommitment uses the
// last matching first-share index instead of the first match. When two blobs in
// the same namespace have identical first shares (same namespace, same sequence
// length, same initial data bytes), ProveCommitment finds the wrong start index
// for the earlier blob, producing an invalid commitment proof.
//
// See: https://github.com/celestiaorg/celestia-node/blob/main/blob/service.go#L672-L677
func TestProveCommitmentCollision(t *testing.T) {
	ns := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0xAB}, 10))

	// Both blobs must span at least 2 shares so their data extends beyond the
	// first share. The first FirstSparseShareContentSize bytes of data are
	// identical so that the first share of each blob is byte-equal.
	firstShareDataSize := libshare.FirstSparseShareContentSize
	extraBytes := 200 // enough to spill into a second share

	// Shared prefix that fills the entire first share's data area.
	sharedPrefix := bytes.Repeat([]byte{0x42}, firstShareDataSize)

	// blob1 data: sharedPrefix + [0xAA ...] in the second share
	data1 := make([]byte, firstShareDataSize+extraBytes)
	copy(data1, sharedPrefix)
	for i := firstShareDataSize; i < len(data1); i++ {
		data1[i] = 0xAA
	}

	// blob2 data: sharedPrefix + [0xBB ...] in the second share
	data2 := make([]byte, firstShareDataSize+extraBytes)
	copy(data2, sharedPrefix)
	for i := firstShareDataSize; i < len(data2); i++ {
		data2[i] = 0xBB
	}

	blob1, err := NewBlob(libshare.ShareVersionZero, ns, data1, nil)
	require.NoError(t, err)
	blob2, err := NewBlob(libshare.ShareVersionZero, ns, data2, nil)
	require.NoError(t, err)

	// Commitments must differ (data differs after the first share).
	require.False(t, bytes.Equal(blob1.Commitment, blob2.Commitment),
		"blobs should have different commitments")

	// Convert each blob to shares independently.
	shares1, err := BlobsToShares(blob1)
	require.NoError(t, err)
	shares2, err := BlobsToShares(blob2)
	require.NoError(t, err)

	// Verify the first shares are indeed byte-equal (precondition for the bug).
	require.True(t, bytes.Equal(shares1[0].ToBytes(), shares2[0].ToBytes()),
		"first shares of both blobs should be byte-equal for this test to be valid")

	// Build an ODS with blob1 shares followed by blob2 shares. Both blobs are
	// in the same namespace so this ordering is valid.
	allShares := append(shares1, shares2...)

	odsSize := int(utils.SquareSize(len(allShares)))
	// Pad with tail padding shares to fill the square.
	for len(allShares) < odsSize*odsSize {
		allShares = append(allShares, libshare.TailPaddingShare())
	}

	eds, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(allShares),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(t, err)

	// Compute the data root from the EDS row/column roots.
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	dataRoot := roots.Hash()

	// --- Prove and verify blob1 (the first blob in the ODS) ---
	proof1, err := ProveCommitment(eds, ns, shares1)
	require.NoError(t, err)
	require.NoError(t, proof1.Validate())

	err = proof1.Verify(dataRoot, blob1.Commitment)
	require.NoError(t, err, "commitment proof for blob1 should verify but doesn't — "+
		"ProveCommitment found the wrong start index (last match instead of first match)")

	// --- Prove and verify blob2 (the second blob in the ODS) ---
	proof2, err := ProveCommitment(eds, ns, shares2)
	require.NoError(t, err)
	require.NoError(t, proof2.Validate())

	err = proof2.Verify(dataRoot, blob2.Commitment)
	require.NoError(t, err, "commitment proof for blob2 should also verify")
}
