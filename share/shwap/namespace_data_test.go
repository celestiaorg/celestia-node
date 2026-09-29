package shwap_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestNamespaceDataReadFromCapsRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	maxRows := share.MaxSquareSize

	// serialize a single valid row to replay as the attacker's payload.
	const odsSize = 8
	namespace := libshare.RandomNamespace()
	randEDS, _ := edstest.RandEDSWithNamespace(t, namespace, odsSize, odsSize)
	nd, err := eds.NamespaceData(ctx, &eds.Rsmt2D{ExtendedDataSquare: randEDS}, namespace)
	require.NoError(t, err)
	require.NotEmpty(t, nd)

	var row bytes.Buffer
	_, err = nd[0].WriteTo(&row)
	require.NoError(t, err)

	// one more row than a namespace can occupy in the largest possible ODS.
	var stream bytes.Buffer
	for range maxRows + 1 {
		stream.Write(row.Bytes())
	}

	var got shwap.NamespaceData
	_, err = got.ReadFrom(&stream)
	require.Error(t, err)
	require.Empty(t, got, "no rows must be committed when the cap is exceeded")
}

// TestNamespaceDataReadFromRoundTrip ensures a well-formed response of legitimate
// size still round-trips through WriteTo/ReadFrom unchanged.
func TestNamespaceDataReadFromRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	const odsSize = 8
	namespace := libshare.RandomNamespace()
	randEDS, _ := edstest.RandEDSWithNamespace(t, namespace, odsSize*odsSize/2, odsSize)
	nd, err := eds.NamespaceData(ctx, &eds.Rsmt2D{ExtendedDataSquare: randEDS}, namespace)
	require.NoError(t, err)
	require.NotEmpty(t, nd)

	var buf bytes.Buffer
	_, err = nd.WriteTo(&buf)
	require.NoError(t, err)

	var got shwap.NamespaceData
	_, err = got.ReadFrom(&buf)
	require.NoError(t, err)
	require.Equal(t, nd, got)
}

// TestNamespaceDataIDResponseReader ensures the row-by-row streamed response is byte-identical
// to NamespaceData.WriteTo and verifies against the roots, for both a present namespace and an
// absent one (absence proofs).
func TestNamespaceDataIDResponseReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	const odsSize = 8
	present := libshare.RandomNamespace()
	randEDS, roots := edstest.RandEDSWithNamespace(t, present, odsSize*odsSize/2, odsSize)
	acc := &eds.Rsmt2D{ExtendedDataSquare: randEDS}

	// a namespace that is absent but still lands inside the square, so the case exercises absence
	// proofs. A namespace outside it maps to no rows at all and would assert nothing.
	absent := libshare.RandomNamespace()
	for {
		rowIdxs, err := share.RowsWithNamespace(roots, absent)
		require.NoError(t, err)
		if len(rowIdxs) > 0 {
			break
		}
		absent = libshare.RandomNamespace()
	}

	for name, namespace := range map[string]libshare.Namespace{
		"present": present,
		"absent":  absent,
	} {
		t.Run(name, func(t *testing.T) {
			nd, err := eds.NamespaceData(ctx, acc, namespace)
			require.NoError(t, err)

			var want bytes.Buffer
			_, err = nd.WriteTo(&want)
			require.NoError(t, err)

			id, err := shwap.NewNamespaceDataID(1, namespace)
			require.NoError(t, err)
			r, err := id.ResponseReader(ctx, acc)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, r.(io.Closer).Close()) })

			var got bytes.Buffer
			_, err = got.ReadFrom(r)
			require.NoError(t, err)
			require.Equal(t, want.Bytes(), got.Bytes())

			var decoded shwap.NamespaceData
			_, err = decoded.ReadFrom(&got)
			require.NoError(t, err)
			require.NoError(t, decoded.Verify(roots, namespace))
		})
	}
}

// TestNamespaceDataIDResponseReaderCancel ensures Close stops a partially consumed stream and a
// cancelled reader returns an error instead of blocking.
func TestNamespaceDataIDResponseReaderCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	const odsSize = 16
	namespace := libshare.RandomNamespace()
	randEDS, _ := edstest.RandEDSWithNamespace(t, namespace, odsSize*odsSize, odsSize)
	acc := &eds.Rsmt2D{ExtendedDataSquare: randEDS}

	id, err := shwap.NewNamespaceDataID(1, namespace)
	require.NoError(t, err)

	readCtx, readCancel := context.WithCancel(ctx)
	r, err := id.ResponseReader(readCtx, acc)
	require.NoError(t, err)

	// the square has more rows than the prefetch window, so the tail is never fetched and the
	// cancelled reader has to report an error rather than serve a short stream.
	// consume the first row only, then cancel mid-stream.
	_, err = r.Read(make([]byte, 1))
	require.NoError(t, err)
	readCancel()

	_, err = io.ReadAll(r)
	require.Error(t, err)
	require.NoError(t, r.(io.Closer).Close())
}
