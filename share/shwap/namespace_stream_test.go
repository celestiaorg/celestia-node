package shwap_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestNamespaceResponseWireFormat(t *testing.T) {
	ctx := context.Background()
	ns := libshare.RandomNamespace()
	square, roots := edstest.RandEDSWithNamespace(t, ns, 16, 4)
	acc := &eds.Rsmt2D{ExtendedDataSquare: square}
	data, err := eds.NamespaceData(ctx, acc, ns)
	require.NoError(t, err)
	var want bytes.Buffer
	_, err = data.WriteTo(&want)
	require.NoError(t, err)
	id, err := shwap.NewNamespaceDataID(1, ns)
	require.NoError(t, err)
	r, err := id.ResponseReader(ctx, acc)
	require.NoError(t, err)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, want.Bytes(), got)
	var decoded shwap.NamespaceData
	_, err = decoded.ReadFrom(bytes.NewReader(got))
	require.NoError(t, err)
	require.NoError(t, decoded.Verify(roots, ns))
}

func TestRangeResponseWireFormat(t *testing.T) {
	ctx := context.Background()
	ns := libshare.RandomNamespace()
	square, _ := edstest.RandEDSWithNamespace(t, ns, 16, 4)
	acc := &eds.Rsmt2D{ExtendedDataSquare: square}
	eid, err := shwap.NewEdsID(1)
	require.NoError(t, err)
	for from := range 16 {
		for to := from + 1; to <= 16; to++ {
			data, err := acc.RangeNamespaceData(ctx, from, to)
			require.NoError(t, err)
			var want bytes.Buffer
			_, err = data.WriteTo(&want)
			require.NoError(t, err)
			id, err := shwap.NewRangeNamespaceDataID(eid, from, to, 4)
			require.NoError(t, err)
			r, err := id.ResponseReader(ctx, acc)
			require.NoError(t, err)
			got, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, want.Bytes(), got, "range [%d,%d)", from, to)
		}
	}
}

func TestRangeResponseRejectsInvalidRange(t *testing.T) {
	ctx := context.Background()
	ns := libshare.RandomNamespace()
	square, _ := edstest.RandEDSWithNamespace(t, ns, 8, 4)
	acc := &eds.Rsmt2D{ExtendedDataSquare: square}
	eid, err := shwap.NewEdsID(1)
	require.NoError(t, err)
	for _, bounds := range [][2]int{{-1, 1}, {0, 0}, {2, 1}, {0, 17}} {
		id := shwap.RangeNamespaceDataID{EdsID: eid, From: bounds[0], To: bounds[1]}
		_, err := id.ResponseReader(ctx, acc)
		require.Error(t, err)
	}
	id, err := shwap.NewRangeNamespaceDataID(eid, 0, 16, 4)
	require.NoError(t, err)
	r, err := id.ResponseReader(ctx, acc)
	if err == nil {
		_, err = io.Copy(io.Discard, r)
	}
	require.ErrorContains(t, err, "mismatched namespace")
}
