package share

import (
	"bytes"
	mrand "math/rand"
	"sort"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"
)

// EqualEDS check whether two given EDSes are equal.
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

// RandEDS generates EDS filled with the random data with the given size for original square. It uses require.TestingT
// to be able to take both a *testing.T and a *testing.B.
func RandEDS(t require.TestingT, size int) *rsmt2d.ExtendedDataSquare {
	shares := RandShares(t, size*size)
	// recompute the eds
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(size)))
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandShares generate 'total' amount of shares filled with random data. It uses require.TestingT to be able to take
// both a *testing.T and a *testing.B.
func RandShares(t require.TestingT, total int) []Share {
	if total&(total-1) != 0 {
		t.Errorf("Namespace total must be power of 2: %d", total)
		t.FailNow()
	}

	shares := make([]Share, total)
	for i := range shares {
		nid := make([]byte, Size)
		_, err := mrand.Read(nid[:NamespaceSize])
		require.NoError(t, err)
		shares[i] = nid
	}
	sort.Slice(shares, func(i, j int) bool { return bytes.Compare(shares[i], shares[j]) < 0 })

	for i := range shares {
		_, err := mrand.Read(shares[i][NamespaceSize:])
		require.NoError(t, err)
	}

	return shares
}
