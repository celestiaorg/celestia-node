package ipld

import (
	"bytes"
	"crypto/sha256"
	"math"
	mrand "math/rand"
	"sort"
	"testing"

	"github.com/celestiaorg/rsmt2d"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-core/pkg/wrapper"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
)

// TODO(Wondertan): Move to rsmt2d
// TODO(Wondertan): Propose use of int by default instead of uint for the sake convenience and Golang practices
func EqualEDS(a *rsmt2d.ExtendedDataSquare, b *rsmt2d.ExtendedDataSquare) bool {
	if a.Width() != b.Width() {
		return false
	}

	for i := uint(0); i < a.Width(); i++ {
		ar, br := a.Row(i), b.Row(i)
		for j := 0; j < len(ar); j++ {
			if !bytes.Equal(ar[j], br[j]) {
				return false
			}
		}
	}

	return true
}

// TODO(Wondertan): Move to NMT plugin
func RandNamespacedCID(t *testing.T) cid.Cid {
	raw := make([]byte, NamespaceSize*2+sha256.Size)
	_, err := mrand.Read(raw) // nolint:gosec // G404: Use of weak random number generator
	require.NoError(t, err)
	id, err := plugin.CidFromNamespacedSha256(raw)
	require.NoError(t, err)
	return id
}

func RandEDS(t *testing.T, size int) *rsmt2d.ExtendedDataSquare {
	shares := RandNamespacedShares(t, size*size)
	// create the nmt wrapper to generate row and col commitments
	squareSize := uint32(math.Sqrt(float64(len(shares))))
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(squareSize))
	// recompute the eds
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares.Raw(), rsmt2d.NewRSGF8Codec(), tree.Constructor)
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

func RandNamespacedShares(t *testing.T, total int) NamespacedShares {
	if total&(total-1) != 0 {
		t.Fatal("Namespace total must be power of 2")
	}

	data := make([][]byte, total)
	for i := 0; i < total; i++ {
		nid := make([]byte, NamespaceSize)
		_, err := mrand.Read(nid) // nolint:gosec // G404: Use of weak random number generator
		require.NoError(t, err)
		data[i] = nid
	}
	sortByteArrays(data)

	shares := make(NamespacedShares, total)
	for i := 0; i < total; i++ {
		shares[i].ID = data[i]
		shares[i].Share = make([]byte, NamespaceSize+ShareSize)
		copy(shares[i].Share[:NamespaceSize], data[i])
		_, err := mrand.Read(shares[i].Share[NamespaceSize:]) // nolint:gosec // G404: Use of weak random number generator
		require.NoError(t, err)
	}

	return shares
}

func sortByteArrays(src [][]byte) {
	sort.Slice(src, func(i, j int) bool { return bytes.Compare(src[i], src[j]) < 0 })
}
