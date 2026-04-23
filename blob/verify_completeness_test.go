package blob

import (
	"bytes"
	"crypto/rand"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v9/pkg/wrapper"
	"github.com/celestiaorg/go-square/merkle"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// TestVerifyAcceptsForgedCommitmentWithTrailingRoots demonstrates that
// CommitmentProof.Verify() does not check that all SubtreeRoots were verified
// by the SubtreeRootProofs. An attacker can append arbitrary bytes to
// SubtreeRoots, compute a new commitment from the modified slice, and Verify()
// will accept it against the original data root.
func TestVerifyAcceptsForgedCommitmentWithTrailingRoots(t *testing.T) {
	// Build a real blob, EDS, proof, and data root.
	ns := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0xAB}, 10))
	data := make([]byte, 350)
	_, err := rand.Read(data)
	require.NoError(t, err)

	blob, err := NewBlob(libshare.ShareVersionZero, ns, data, nil)
	require.NoError(t, err)

	blobShares, err := BlobsToShares(blob)
	require.NoError(t, err)

	eds, dataRoot := buildTestEDS(t, blobShares)

	commitmentProof, err := ProveCommitment(eds, ns, blobShares)
	require.NoError(t, err)

	// Sanity: the legitimate proof verifies.
	err = commitmentProof.Verify(dataRoot, blob.Commitment)
	require.NoError(t, err, "legitimate proof must verify")

	// --- ATTACK: append a fake subtree root ---
	fakeRoot := make([]byte, 32)
	_, err = rand.Read(fakeRoot)
	require.NoError(t, err)

	forgedRoots := make([][]byte, len(commitmentProof.SubtreeRoots)+1)
	copy(forgedRoots, commitmentProof.SubtreeRoots)
	forgedRoots[len(forgedRoots)-1] = fakeRoot

	// Compute a forged commitment from the modified subtree roots.
	forgedCommitment := merkle.HashFromByteSlices(forgedRoots)
	// The forged commitment must differ from the real one.
	require.NotEqual(t, blob.Commitment, Commitment(forgedCommitment),
		"forged commitment should differ from legitimate commitment")

	// Mutate the proof in-place.
	commitmentProof.SubtreeRoots = forgedRoots

	// Verify() should reject this, but currently does not.
	err = commitmentProof.Verify(dataRoot, forgedCommitment)
	require.Error(t, err,
		"Verify() should reject a forged commitment with unverified trailing subtree roots")
}

func buildTestEDS(t *testing.T, shares []libshare.Share) (*rsmt2d.ExtendedDataSquare, []byte) {
	t.Helper()
	odsSize := int(math.Ceil(math.Sqrt(float64(len(shares)))))
	if odsSize < 1 {
		odsSize = 1
	}
	for len(shares) < odsSize*odsSize {
		shares = append(shares, libshare.TailPaddingShare())
	}
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(shares),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(t, err)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	return eds, roots.Hash()
}
