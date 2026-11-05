package eds

import (
	"bytes"
	"context"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// FuzzReadAccessor feeds an EDS stream, as a shrex peer sends it in response
// to an EdsID request, to ReadAccessor with the roots of a fixed square. A
// stream that is accepted must reproduce that square exactly.
func FuzzReadAccessor(f *testing.F) {
	const (
		odsSize = 4
		padding = 5
	)
	square, roots := deterministicPaddedSquare(f, odsSize, padding)
	ods := square.FlattenedODS()
	full := bytes.Join(ods, nil)
	// A server may stop at the last non-padding share; the reader pads the rest.
	withoutPadding := bytes.Join(ods[:len(ods)-padding], nil)

	f.Add(full)
	f.Add(withoutPadding)
	f.Add(withoutPadding[:len(withoutPadding)-1])
	f.Add(full[:libshare.ShareSize*odsSize])
	f.Add(append(append([]byte{}, full...), 0))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		acc, err := ReadAccessor(context.Background(), bytes.NewReader(raw), roots)
		if err != nil {
			return
		}
		require.Equal(t, ods, acc.FlattenedODS(), "an accepted stream must reproduce the committed square")
	})
}

// deterministicPaddedSquare builds a square from a fixed-seed generator whose
// last `padding` ODS shares are tail padding.
func deterministicPaddedSquare(tb testing.TB, odsSize, padding int) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
	tb.Helper()
	rng := rand.New(rand.NewChaCha8([32]byte{'e', 'd', 's'}))
	shares := make([]libshare.Share, odsSize*odsSize-padding, odsSize*odsSize)
	for i := range shares {
		raw := make([]byte, libshare.ShareSize)
		id := make([]byte, libshare.NamespaceVersionZeroIDSize)
		for j := range id {
			id[j] = byte(rng.Uint32())
		}
		ns, err := libshare.NewV0Namespace(id)
		require.NoError(tb, err)
		copy(raw[:libshare.NamespaceSize], ns.Bytes())
		for j := libshare.NamespaceSize; j < len(raw); j++ {
			raw[j] = byte(rng.Uint32())
		}
		sh, err := libshare.NewShare(raw)
		require.NoError(tb, err)
		shares[i] = sh
	}
	slices.SortFunc(shares, func(a, b libshare.Share) int { return bytes.Compare(a.ToBytes(), b.ToBytes()) })
	for range padding {
		shares = append(shares, libshare.TailPaddingShare())
	}

	square, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(shares),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(tb, err)
	roots, err := share.NewAxisRoots(square)
	require.NoError(tb, err)
	return square, roots
}
