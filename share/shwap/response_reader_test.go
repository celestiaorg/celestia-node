package shwap_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var errRowRead = errors.New("row read failed")

// failingAccessor fails every row read, standing in for a corrupt or unreadable file.
type failingAccessor struct {
	*eds.Rsmt2D
}

func (failingAccessor) RowNamespaceData(
	context.Context, libshare.Namespace, int,
) (shwap.RowNamespaceData, error) {
	return shwap.RowNamespaceData{}, errRowRead
}

func (failingAccessor) AxisHalf(context.Context, rsmt2d.Axis, int) (shwap.AxisHalf, error) {
	return shwap.AxisHalf{}, errRowRead
}

// TestResponseReaderFailsBeforeStreaming pins the property the shrex server relies on: a reader is
// handed back only once the first row is in hand, so a read error still becomes an INTERNAL status
// instead of a truncated OK response the client would blame its peer for.
func TestResponseReaderFailsBeforeStreaming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	const odsSize = 8
	namespace := libshare.RandomNamespace()
	randEDS, _ := edstest.RandEDSWithNamespace(t, namespace, odsSize*odsSize, odsSize)
	acc := failingAccessor{&eds.Rsmt2D{ExtendedDataSquare: randEDS}}

	t.Run("namespace data", func(t *testing.T) {
		id, err := shwap.NewNamespaceDataID(1, namespace)
		require.NoError(t, err)
		_, err = id.ResponseReader(ctx, acc)
		require.ErrorIs(t, err, errRowRead)
	})

	t.Run("range namespace data", func(t *testing.T) {
		edsID, err := shwap.NewEdsID(1)
		require.NoError(t, err)
		id, err := shwap.NewRangeNamespaceDataID(edsID, 0, odsSize*odsSize, odsSize)
		require.NoError(t, err)
		_, err = id.ResponseReader(ctx, acc)
		require.ErrorIs(t, err, errRowRead)
	})
}
