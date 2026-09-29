package shwap_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// TestRangeNamespaceDataIDResponseReader checks the streamed range is byte-identical to the
// materialized RangeNamespaceData for every shape of range: whole rows, cut start, cut end, cut
// both, single row and many rows.
func TestRangeNamespaceDataIDResponseReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	const odsSize = 4
	namespace := libshare.RandomNamespace()
	randEDS, _ := edstest.RandEDSWithNamespace(t, namespace, odsSize*odsSize, odsSize)
	acc := &eds.Rsmt2D{ExtendedDataSquare: randEDS}

	edsID, err := shwap.NewEdsID(1)
	require.NoError(t, err)

	for from := range odsSize * odsSize {
		for to := from + 1; to <= odsSize*odsSize; to++ {
			id, err := shwap.NewRangeNamespaceDataID(edsID, from, to, odsSize)
			require.NoError(t, err)

			materialized, err := acc.RangeNamespaceData(ctx, from, to)
			require.NoError(t, err)
			want := &bytes.Buffer{}
			_, err = materialized.WriteTo(want)
			require.NoError(t, err)

			r, err := id.ResponseReader(ctx, acc)
			require.NoError(t, err)
			got, err := io.ReadAll(r)
			require.NoError(t, err)
			require.NoError(t, r.(io.Closer).Close())

			require.Equal(t, want.Bytes(), got, "range [%d,%d)", from, to)
		}
	}
}

// TestRangeNamespaceDataIDResponseReaderVerifies checks the streamed bytes decode and verify
// against the roots, so a client still accepts what the server sends.
func TestRangeNamespaceDataIDResponseReaderVerifies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	const odsSize = 8
	namespace := libshare.RandomNamespace()
	randEDS, roots := edstest.RandEDSWithNamespace(t, namespace, odsSize*odsSize, odsSize)
	acc := &eds.Rsmt2D{ExtendedDataSquare: randEDS}

	edsID, err := shwap.NewEdsID(1)
	require.NoError(t, err)

	for _, tc := range []struct{ from, to int }{
		{from: 0, to: odsSize * odsSize},
		{from: 3, to: 20},
		{from: 1, to: 4},
	} {
		id, err := shwap.NewRangeNamespaceDataID(edsID, tc.from, tc.to, odsSize)
		require.NoError(t, err)

		r, err := id.ResponseReader(ctx, acc)
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.(io.Closer).Close())

		var decoded shwap.RangeNamespaceData
		_, err = decoded.ReadFrom(bytes.NewReader(got))
		require.NoError(t, err)

		from, err := shwap.SampleCoordsFrom1DIndex(tc.from, odsSize)
		require.NoError(t, err)
		to, err := shwap.SampleCoordsFrom1DIndex(tc.to-1, odsSize)
		require.NoError(t, err)
		require.NoError(t, decoded.VerifyInclusion(from, to, odsSize, roots.RowRoots[from.Row:to.Row+1]))
	}
}
