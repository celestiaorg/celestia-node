package shwap_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestRangeNamespaceData(t *testing.T) {
	const (
		odsSize = 16
		edsSize = odsSize * odsSize
	)

	ns := libshare.RandomNamespace()
	square, root := edstest.RandEDSWithNamespace(t, ns, odsSize, odsSize)

	nsRowStart := -1
	for i, row := range root.RowRoots {
		outside, err := share.IsOutsideRange(ns, row, row)
		require.NoError(t, err)
		if !outside {
			nsRowStart = i
			break
		}
	}
	assert.Greater(t, nsRowStart, -1)

	extended := &eds.Rsmt2D{ExtendedDataSquare: square}

	nsData, err := extended.RowNamespaceData(context.Background(), ns, nsRowStart)
	require.NoError(t, err)
	col := nsData.Proof.Start()

	rowRoots, err := extended.RowRoots()
	require.NoError(t, err)
	colRoots, err := extended.ColRoots()
	require.NoError(t, err)
	dataHash, err := extended.DataHash(context.Background())
	require.NoError(t, err)

	for i := 1; i <= odsSize; i++ {
		t.Run(fmt.Sprintf("range of %d shares", i), func(t *testing.T) {
			toRow, toCol := nsRowStart, col+i-1
			for toCol >= odsSize {
				toRow++
				toCol -= odsSize
			}

			to := shwap.SampleCoords{Row: toRow, Col: toCol}
			dataID, err := shwap.NewRangeNamespaceDataID(
				1,
				ns,
				shwap.SampleCoords{Row: nsRowStart, Col: col},
				to,
				edsSize,
			)
			require.NoError(t, err)

			rngdata, err := extended.RangeNamespaceData(
				context.Background(),
				dataID.DataNamespace,
				shwap.SampleCoords{Row: dataID.RowIndex, Col: dataID.ShareIndex},
				to,
			)
			require.NoError(t, err)

			roots, err := extended.AxisRoots(context.Background())
			require.NoError(t, err)

			err = rngdata.Validate(roots, &dataID)
			require.NoError(t, err)
			proof := rngdata.ProveRange(nsRowStart, rowRoots, colRoots)
			err = proof.Validate(dataHash)
			require.NoError(t, err)
		})
	}
}
