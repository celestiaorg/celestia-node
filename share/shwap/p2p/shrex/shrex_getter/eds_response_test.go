package shrex_getter //nolint:stylecheck // underscore in pkg name will be fixed with shrex refactoring

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestEDSResponse_ReadFrom(t *testing.T) {
	const odsSize = 4
	ctx := context.Background()

	square := edstest.RandEDS(t, odsSize)
	roots, err := share.NewAxisRoots(square)
	require.NoError(t, err)

	reader, err := (&eds.Rsmt2D{ExtendedDataSquare: square}).Reader()
	require.NoError(t, err)

	resp := &edsResponse{ctx: ctx, root: roots}
	n, err := resp.ReadFrom(reader)
	require.NoError(t, err)

	require.Equal(t, int64(odsSize*odsSize*libshare.ShareSize), n)
	require.NotNil(t, resp.eds)
	require.True(t, square.Equals(resp.eds.ExtendedDataSquare))
}

func TestEDSResponse_ReadFrom_RootMismatch(t *testing.T) {
	const odsSize = 4
	ctx := context.Background()

	square := edstest.RandEDS(t, odsSize)
	reader, err := (&eds.Rsmt2D{ExtendedDataSquare: square}).Reader()
	require.NoError(t, err)

	// roots of an unrelated square make ReadAccessor fail the integrity check
	otherRoots, err := share.NewAxisRoots(edstest.RandEDS(t, odsSize))
	require.NoError(t, err)

	resp := &edsResponse{ctx: ctx, root: otherRoots}
	_, err = resp.ReadFrom(reader)
	require.Error(t, err)
	require.Nil(t, resp.eds)
}

func TestCountingReader(t *testing.T) {
	data := bytes.Repeat([]byte{0xab}, 1<<16+7)
	cr := &countingReader{r: bytes.NewReader(data)}

	out, err := io.ReadAll(cr)
	require.NoError(t, err)
	require.Equal(t, data, out)
	require.Equal(t, int64(len(data)), cr.n)
}
