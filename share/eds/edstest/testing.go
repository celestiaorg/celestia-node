package edstest

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	appshares "github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func RandByzantineEDS(t testing.TB, odsSize int, options ...nmt.Option) *rsmt2d.ExtendedDataSquare {
	eds := RandEDS(t, odsSize)
	shares := eds.Flattened()
	copy(share.GetData(shares[0]), share.GetData(shares[1])) // corrupting eds
	eds, err := rsmt2d.ImportExtendedDataSquare(shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize),
			options...))
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandEDS generates EDS filled with the random data with the given size for original square.
func RandEDS(t testing.TB, odsSize int) *rsmt2d.ExtendedDataSquare {
	shares := sharetest.RandShares(t, odsSize*odsSize)
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, share.DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(odsSize)))
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandEDSWithTailPadding generates EDS of given ODS size filled with randomized and tail padding shares.
func RandEDSWithTailPadding(t testing.TB, odsSize, padding int) *rsmt2d.ExtendedDataSquare {
	shares := sharetest.RandShares(t, odsSize*odsSize)
	for i := len(shares) - padding; i < len(shares); i++ {
		paddingShare := appshares.TailPaddingShare()
		shares[i] = paddingShare.ToBytes()
	}

	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, share.DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(odsSize)))
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandEDSWithNamespace generates EDS with given square size. Returned EDS will have
// namespacedAmount of shares with the given namespace.
func RandEDSWithNamespace(
	t testing.TB,
	namespace share.Namespace,
	namespacedAmount, odsSize int,
) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
	shares := sharetest.RandSharesWithNamespace(t, namespace, namespacedAmount, odsSize*odsSize)
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, share.DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(odsSize)))
	require.NoError(t, err, "failure to recompute the extended data square")
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	return eds, roots
}

// RandomAxisRoots generates random share.AxisRoots for the given eds size.
func RandomAxisRoots(t testing.TB, edsSize int) *share.AxisRoots {
	roots := make([][]byte, edsSize*2)
	for i := range roots {
		root := make([]byte, edsSize)
		_, err := rand.Read(root)
		require.NoError(t, err)
		roots[i] = root
	}

	rows := roots[:edsSize]
	cols := roots[edsSize:]
	return &share.AxisRoots{
		RowRoots:    rows,
		ColumnRoots: cols,
	}
}
