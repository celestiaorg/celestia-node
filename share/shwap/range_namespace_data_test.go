package shwap_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/rand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/store/file"
)

func TestRangeNamespaceData(t *testing.T) {
	const (
		odsSize      = 16
		sharesAmount = odsSize * odsSize
	)
	square, root := edstest.RandEDSWithNamespace(t, libshare.RandomNamespace(), sharesAmount, odsSize)

	extended := &eds.Rsmt2D{ExtendedDataSquare: square}
	nsRowStart := 0
	nsColStart := 0

	path := t.TempDir() + "/" + strconv.Itoa(rand.Intn(1000))
	err := file.CreateODS(path, root, square)
	require.NoError(t, err)
	ods, err := file.OpenODS(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ods.Close())
	})
	t.Logf("ns started at - row:%d col:%d ", nsRowStart, nsColStart)
	// 36k test cases
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
					t.Run(fmt.Sprintf("EDS:%s", str), func(t *testing.T) {
						rngdata, err := extended.RangeNamespaceData(context.Background(), fromIndex, toIndex+1)
						require.NoError(t, err)
						err = rngdata.VerifyInclusion(
							from, to,
							len(root.RowRoots)/2,
							root.RowRoots[from.Row:to.Row+1],
						)
						require.NoError(t, err)
						data := rngdata.Flatten()
						assert.Len(t, data, toIndex-fromIndex+1)
					})
					t.Run(fmt.Sprintf("ODS:%s", str), func(t *testing.T) {
						rngdata, err := ods.RangeNamespaceData(context.Background(), fromIndex, toIndex+1)
						require.NoError(t, err)
						err = rngdata.VerifyInclusion(
							from, to,
							len(root.RowRoots)/2,
							root.RowRoots[from.Row:to.Row+1],
						)
						require.NoError(t, err)
						data := rngdata.Flatten()
						assert.Len(t, data, toIndex-fromIndex+1)
					})
				}
			}
		}
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
		rngData, err := extended.RangeNamespaceData(ctx, 0, length)
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
	extended := &eds.Rsmt2D{ExtendedDataSquare: square}
	accessor := eds.WithValidation(extended)
	from := shwap.SampleCoords{Row: 0, Col: 0}
	to := shwap.SampleCoords{Row: odsSize - 1, Col: odsSize - 1}
	rngdata, err := accessor.RangeNamespaceData(
		context.Background(),
		0,
		odsSize*odsSize,
	)
	require.NoError(t, err)
	err = rngdata.VerifyInclusion(from, to, len(root.RowRoots)/2, root.RowRoots[from.Row:to.Row+1])
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
	assert.Equal(t, rngdata, rangeNsData)
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
