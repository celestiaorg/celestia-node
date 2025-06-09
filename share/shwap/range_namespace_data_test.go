package shwap_test

import (
	"context"
	"encoding/json"
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
	square, root := edstest.RandEDSWithNamespace(t, ns, sharesAmount, odsSize)

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

			err = rngdata.VerifyShares(rngdata.Shares, ns, dataID.From, dataID.To, root.Hash())
			require.NoError(t, err)
		})
	}
}

func TestRangeNamespaceDataV1(t *testing.T) {
	const (
		odsSize      = 16
		sharesAmount = odsSize * odsSize
	)

	ns := libshare.RandomNamespace()
	square, root := edstest.RandEDSWithNamespace(t, ns, sharesAmount, odsSize)

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
	nsColStart := nsData.Proof.Start()

	t.Logf("ns started at - row:%d col:%d ", nsRowStart, nsColStart)
	for fromRow := nsRowStart; fromRow < odsSize; fromRow++ {
		for fromCol := nsColStart; fromCol < odsSize; fromCol++ {
			for toRow := odsSize - 1; toRow >= fromRow; toRow-- {
				for toCol := odsSize - 1; toCol >= fromCol; toCol-- {
					from := shwap.SampleCoords{Row: fromRow, Col: fromCol}
					to := shwap.SampleCoords{Row: toRow, Col: toCol}
					fromIndex, err := shwap.SampleCoordsAs1DIndex(from, odsSize)
					require.NoError(t, err)
					toIndex, err := shwap.SampleCoordsAs1DIndex(to, odsSize)
					require.NoError(t, err)
					str := fmt.Sprintf(
						"range with coordinate from [%d;%d] to[%d;%d]. Number of shares:%d",
						fromRow, fromCol, toRow, toCol, toIndex-fromIndex+1,
					)
					rawShares := make([][]libshare.Share, 0, toRow-fromRow+1)
					for row := fromRow; row <= toRow; row++ {
						rawShare := extended.Row(uint(row))
						sh, err := libshare.FromBytes(rawShare)
						require.NoError(t, err)
						rawShares = append(rawShares, sh)
					}

					t.Run(str, func(t *testing.T) {
						rngdata, err := shwap.RangeNamespaceDataFromSharesV1(rawShares, ns, root, from, to)
						require.NoError(t, err)
						err = rngdata.VerifySharesV1(rngdata.Shares, ns, from, to, root.Hash())
						require.NoError(t, err)
					})
				}
			}
		}
	}
}

func TestRangeNamespaceDataV1_FullODS(t *testing.T) {
	const (
		odsSize      = 4
		sharesAmount = odsSize * odsSize
	)

	ns := libshare.RandomNamespace()
	square, root := edstest.RandEDSWithNamespace(t, ns, sharesAmount, odsSize)

	from := shwap.SampleCoords{Row: 0, Col: 0}
	to := shwap.SampleCoords{Row: odsSize - 1, Col: odsSize - 1}
	rawShares := make([][]libshare.Share, 0, odsSize)
	for row := 0; row < odsSize; row++ {
		rawShare := square.Row(uint(row))
		sh, err := libshare.FromBytes(rawShare)
		require.NoError(t, err)
		rawShares = append(rawShares, sh)
	}
	rngdata, err := shwap.RangeNamespaceDataFromSharesV1(rawShares, ns, root, from, to)
	require.NoError(t, err)

	assert.Len(t, rngdata.Shares, odsSize)
	for _, rowShare := range rngdata.Shares {
		assert.Len(t, rowShare, odsSize)
	}
	require.NoError(t, rngdata.VerifySharesV1(rawShares, ns, from, to, root.Hash()))
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

func TestRangeNamespaceDataMarshalUnmarshal(t *testing.T) {
	const (
		odsSize      = 4
		sharesAmount = odsSize * odsSize
	)

	ns := libshare.RandomNamespace()
	square, root := edstest.RandEDSWithNamespace(t, ns, sharesAmount, odsSize)
	eds := eds.Rsmt2D{ExtendedDataSquare: square}

	from := shwap.SampleCoords{Row: 0, Col: 0}
	to := shwap.SampleCoords{Row: odsSize - 1, Col: odsSize - 1}
	rngdata, err := eds.RangeNamespaceData(
		context.Background(),
		ns,
		from,
		to,
	)
	require.NoError(t, err)
	err = rngdata.Verify(ns, from, to, root.Hash())
	require.NoError(t, err)

	data, err := json.Marshal(rngdata)
	require.NoError(t, err)

	newData := &shwap.RangeNamespaceData{}
	err = json.Unmarshal(data, newData)
	require.NoError(t, err)
	assert.Equal(t, rngdata, *newData)

	pbData := newData.ToProto()
	rangeNsData, err := shwap.RangeNamespaceDataFromProto(pbData)
	require.NoError(t, err)
	assert.Equal(t, rngdata, *rangeNsData)
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
