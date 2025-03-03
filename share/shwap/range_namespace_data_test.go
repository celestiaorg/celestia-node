package shwap_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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
		odsSize      = 16
		sharesAmount = odsSize * odsSize
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

	for i := 1; i <= odsSize; i++ {
		t.Run(fmt.Sprintf("range of %d shares", i), func(t *testing.T) {
			toRow, toCol := nsRowStart, col+i-1
			for toCol >= odsSize {
				toRow++
				toCol -= odsSize
			}

			to := shwap.SampleCoords{Row: toRow, Col: toCol}
			dataID, err := shwap.NewRangeNamespaceDataID(
				shwap.EdsID{Height: 1},
				ns,
				shwap.SampleCoords{Row: nsRowStart, Col: col},
				to,
				sharesAmount,
				false,
			)
			require.NoError(t, err)

			rngdata, err := extended.RangeNamespaceData(
				context.Background(),
				dataID.DataNamespace,
				shwap.SampleCoords{Row: dataID.From.Row, Col: dataID.From.Col},
				to,
			)
			require.NoError(t, err)
			err = rngdata.Verify(ns, dataID.From, dataID.To, root.Hash())
			require.NoError(t, err)
		})
	}
}

func TestRangeCoordsFromIdx(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)
	const (
		odsSize = 4
		edsSize = odsSize * 2
	)

	ns := libshare.RandomNamespace()
	square, _ := edstest.RandEDSWithNamespace(t, ns, odsSize*odsSize, odsSize)
	rngLengths := []int{2, 7, 11, 15}
	extended := &eds.Rsmt2D{ExtendedDataSquare: square}
	for _, length := range rngLengths {
		from, to, err := shwap.RangeCoordsFromIdx(0, length, edsSize)
		require.NoError(t, err)
		rngData, err := extended.RangeNamespaceData(ctx, ns, from, to)
		require.NoError(t, err)
		require.Equal(t, length, len(rngData.Flatten()))
	}
}

func FuzzRangeCoordsFromIdx(f *testing.F) {
	if testing.Short() {
		f.Skip()
	}

	const (
		odsSize = 16
		edsSize = odsSize * 2
	)

	square := edstest.RandEDS(f, odsSize)
	shrs := square.FlattenedODS()
	assert.Equal(f, len(shrs), odsSize*odsSize)

	f.Add(0, 3, edsSize)
	f.Add(10, 14, edsSize)
	f.Add(23, 30, edsSize)
	f.Add(62, 3, edsSize)

	f.Fuzz(func(t *testing.T, edsIndex, length, size int) {
		if edsIndex < 0 || edsIndex >= edsSize*edsSize {
			return
		}

		coords, err := shwap.SampleCoordsFrom1DIndex(edsIndex, edsSize)
		require.NoError(t, err)
		if coords.Row >= odsSize || coords.Col >= odsSize {
			return
		}

		odsIndexStart := coords.Row*odsSize + coords.Col
		if odsIndexStart+length >= odsSize*odsSize {
			return
		}

		if length <= 0 || length >= odsSize*odsSize {
			return
		}
		if size != edsSize {
			return
		}

		from, to, err := shwap.RangeCoordsFromIdx(edsIndex, length, size)
		require.NoError(t, err)
		edsStartShare := square.GetCell(uint(from.Row), uint(from.Col))
		edsEndShare := square.GetCell(uint(to.Row), uint(to.Col))

		odsIndexStart = from.Row*odsSize + from.Col
		odsIndexEnd := to.Row*odsSize + to.Col

		odsStartShare := shrs[odsIndexStart]
		odsEndShare := shrs[odsIndexEnd]
		require.Equal(t, edsStartShare, odsStartShare)
		require.Equal(t, edsEndShare, odsEndShare)
	})
}
