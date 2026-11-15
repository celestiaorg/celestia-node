package blob

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/proof"
	"github.com/celestiaorg/celestia-app/v10/pkg/wrapper"
	"github.com/celestiaorg/go-square/merkle"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/nmt"
)

// TestCommitmentProofVerifyShapes checks that honest proofs verify for blobs of
// different sizes and positions in the square.
func TestCommitmentProofVerifyShapes(t *testing.T) {
	ns := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x11}, 10))
	other := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x01}, 10))

	tests := []struct {
		name       string
		prefixLen  int // bytes of a blob placed before the proven blob
		dataLen    int
		squareSize int
	}{
		{name: "single share at start", dataLen: 100, squareSize: 4},
		{name: "partial row", dataLen: 1000, squareSize: 4},
		{name: "exactly one row", dataLen: 1800, squareSize: 4},
		{name: "two full rows", dataLen: 3500, squareSize: 4},
		{name: "offset single row", prefixLen: 100, dataLen: 1000, squareSize: 8},
		{name: "offset spanning rows", prefixLen: 1000, dataLen: 3000, squareSize: 8},
		{name: "offset spanning many rows", prefixLen: 2500, dataLen: 15000, squareSize: 8},
		{name: "large blob", dataLen: 100_000, squareSize: 32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var shares []libshare.Share
			if tc.prefixLen > 0 {
				prefix, err := NewBlob(libshare.ShareVersionZero, other, bytes.Repeat([]byte{0x01}, tc.prefixLen), nil)
				require.NoError(t, err)
				prefixShares, err := BlobsToShares(prefix)
				require.NoError(t, err)
				shares = append(shares, prefixShares...)
			}
			blob, err := NewBlob(libshare.ShareVersionZero, ns, bytes.Repeat([]byte{0x22}, tc.dataLen), nil)
			require.NoError(t, err)
			blobShares, err := BlobsToShares(blob)
			require.NoError(t, err)
			shares = append(shares, blobShares...)
			require.LessOrEqual(t, len(shares), tc.squareSize*tc.squareSize)
			for len(shares) < tc.squareSize*tc.squareSize {
				shares = append(shares, libshare.TailPaddingShare())
			}

			eds, dataRoot := buildTestEDS(t, shares)
			cp, err := ProveCommitment(eds, ns, blobShares)
			require.NoError(t, err)
			require.NoError(t, cp.Verify(dataRoot, blob.Commitment))
		})
	}
}

// TestCommitmentProofVerifyRejectsForgedLayout checks that a proof built from
// genuine row and subtree proofs is rejected when the rows are repeated or
// reordered, or when the shares are not a contiguous range of the square.
func TestCommitmentProofVerifyRejectsForgedLayout(t *testing.T) {
	nsA := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x11}, 10))
	nsB := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x22}, 10))

	data := bytes.Repeat([]byte{0xA1}, 400)
	data = append(data, bytes.Repeat([]byte{0xA2}, 400)...)
	data = append(data, bytes.Repeat([]byte{0xA3}, 400)...)
	data = append(data, bytes.Repeat([]byte{0xA4}, 400)...)
	blobA, err := NewBlob(libshare.ShareVersionZero, nsA, data, nil)
	require.NoError(t, err)
	sharesA, err := BlobsToShares(blobA)
	require.NoError(t, err)
	require.Len(t, sharesA, 4)

	eds, dataRoot := buildTestEDS(t, sharesA)
	legit, err := ProveCommitment(eds, nsA, sharesA)
	require.NoError(t, err)
	require.NoError(t, legit.Verify(dataRoot, blobA.Commitment))

	rowProve := func(row uint, start, end int) *nmt.Proof {
		tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(eds.Width()/2), row)
		for _, cell := range eds.Row(row) {
			require.NoError(t, tree.Push(cell))
		}
		p, err := tree.ProveRange(start, end)
		require.NoError(t, err)
		return &p
	}
	subtreeRoot := func(idx, start, end, offset int) []byte {
		r, err := computeSubtreeRoots(sharesA[idx:idx+1], []nmt.LeafRange{{Start: start, End: end}}, offset)
		require.NoError(t, err)
		return r[0]
	}

	// Permute the continuation shares [S0, S2, S1, S3] while pointing each
	// subtree root at its genuine position.
	roots := [][]byte{subtreeRoot(0, 0, 1, 0), subtreeRoot(2, 0, 1, 0), subtreeRoot(1, 1, 2, 1), subtreeRoot(3, 1, 2, 1)}
	rowRoots := legit.RowProof.RowRoots
	rowProofs := legit.RowProof.Proofs
	forgedCommitment := merkle.HashFromByteSlices(roots)
	require.NotEqual(t, []byte(blobA.Commitment), forgedCommitment)

	tests := map[string]*CommitmentProof{
		"permuted shares with repeated rows and foreign namespace": {
			SubtreeRoots:      roots,
			SubtreeRootProofs: []*nmt.Proof{rowProve(0, 0, 1), rowProve(1, 0, 1), rowProve(0, 1, 2), rowProve(1, 1, 2)},
			NamespaceID:       nsB.ID(),
			NamespaceVersion:  nsB.Version(),
			RowProof: proof.RowProof{
				RowRoots: [][]byte{rowRoots[0], rowRoots[1], rowRoots[0], rowRoots[1]},
				Proofs:   []*proof.Proof{rowProofs[0], rowProofs[1], rowProofs[0], rowProofs[1]},
				StartRow: 0,
				EndRow:   3,
			},
		},
		"permuted shares with repeated rows": {
			SubtreeRoots:      roots,
			SubtreeRootProofs: []*nmt.Proof{rowProve(0, 0, 1), rowProve(1, 0, 1), rowProve(0, 1, 2), rowProve(1, 1, 2)},
			NamespaceID:       nsA.ID(),
			NamespaceVersion:  nsA.Version(),
			RowProof: proof.RowProof{
				RowRoots: [][]byte{rowRoots[0], rowRoots[1], rowRoots[0], rowRoots[1]},
				Proofs:   []*proof.Proof{rowProofs[0], rowProofs[1], rowProofs[0], rowProofs[1]},
				StartRow: 0,
				EndRow:   3,
			},
		},
		"swapped rows": {
			SubtreeRoots:      append(append([][]byte{}, legit.SubtreeRoots[1]), legit.SubtreeRoots[0]),
			SubtreeRootProofs: []*nmt.Proof{legit.SubtreeRootProofs[1], legit.SubtreeRootProofs[0]},
			NamespaceID:       nsA.ID(),
			NamespaceVersion:  nsA.Version(),
			RowProof: proof.RowProof{
				RowRoots: [][]byte{rowRoots[1], rowRoots[0]},
				Proofs:   []*proof.Proof{rowProofs[1], rowProofs[0]},
				StartRow: 0,
				EndRow:   1,
			},
		},
		"non-contiguous shares across rows": {
			SubtreeRoots:      [][]byte{subtreeRoot(0, 0, 1, 0), subtreeRoot(3, 1, 2, 1)},
			SubtreeRootProofs: []*nmt.Proof{rowProve(0, 0, 1), rowProve(1, 1, 2)},
			NamespaceID:       nsA.ID(),
			NamespaceVersion:  nsA.Version(),
			RowProof: proof.RowProof{
				RowRoots: [][]byte{rowRoots[0], rowRoots[1]},
				Proofs:   []*proof.Proof{rowProofs[0], rowProofs[1]},
				StartRow: 0,
				EndRow:   1,
			},
		},
	}

	for name, cp := range tests {
		t.Run(name, func(t *testing.T) {
			commitment := merkle.HashFromByteSlices(cp.SubtreeRoots)
			require.Error(t, cp.Verify(dataRoot, commitment))
		})
	}
}

// TestCommitmentProofVerifyRejectsWrongNamespace checks that a genuine proof is
// rejected when it claims a namespace other than the one of its shares.
func TestCommitmentProofVerifyRejectsWrongNamespace(t *testing.T) {
	ns := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x11}, 10))
	blob, err := NewBlob(libshare.ShareVersionZero, ns, bytes.Repeat([]byte{0x33}, 1000), nil)
	require.NoError(t, err)
	shares, err := BlobsToShares(blob)
	require.NoError(t, err)
	eds, dataRoot := buildTestEDS(t, shares)

	cp, err := ProveCommitment(eds, ns, shares)
	require.NoError(t, err)
	require.NoError(t, cp.Verify(dataRoot, blob.Commitment))

	other := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x22}, 10))
	cp.NamespaceID = other.ID()
	require.Error(t, cp.Verify(dataRoot, blob.Commitment))

	cp.NamespaceID = nil
	require.Error(t, cp.Verify(dataRoot, blob.Commitment))

	cp.NamespaceID = ns.ID()
	cp.NamespaceVersion = 7
	require.Error(t, cp.Verify(dataRoot, blob.Commitment))
}
