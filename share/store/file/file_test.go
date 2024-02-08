package file

import (
	"context"
	"fmt"
	mrand "math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

type createFile func(eds *rsmt2d.ExtendedDataSquare) EdsFile

func testFileShare(t *testing.T, createFile createFile, size int) {
	eds := edstest.RandEDS(t, size)
	fl := createFile(eds)

	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	width := int(eds.Width())
	for x := 0; x < width; x++ {
		for y := 0; y < width; y++ {
			shr, err := fl.Share(context.TODO(), x, y)
			require.NoError(t, err)

			var axishash []byte
			if shr.Axis == rsmt2d.Row {
				require.Equal(t, getAxis(eds, shr.Axis, y)[x], shr.Share)
				axishash = root.RowRoots[y]
			} else {
				require.Equal(t, getAxis(eds, shr.Axis, x)[y], shr.Share)
				axishash = root.ColumnRoots[x]
			}

			ok := shr.Validate(axishash, x, y, width)
			require.True(t, ok)
		}
	}
}

func testFileData(t *testing.T, createFile createFile, size int) {
	t.Run("included", func(t *testing.T) {
		// generate EDS with random data and some shares with the same namespace
		namespace := sharetest.RandV0Namespace()
		amount := mrand.Intn(size*size-1) + 1
		eds, dah := edstest.RandEDSWithNamespace(t, namespace, amount, size)
		f := createFile(eds)
		testData(t, f, namespace, dah)
	})

	t.Run("not included", func(t *testing.T) {
		// generate EDS with random data and some shares with the same namespace
		eds := edstest.RandEDS(t, size)
		dah, err := share.NewRoot(eds)
		require.NoError(t, err)

		maxNs := nmt.MaxNamespace(dah.RowRoots[(len(dah.RowRoots))/2-1], share.NamespaceSize)
		targetNs, err := share.Namespace(maxNs).AddInt(-1)
		require.NoError(t, err)

		f := createFile(eds)
		testData(t, f, targetNs, dah)
	})
}

func testData(t *testing.T, f EdsFile, namespace share.Namespace, dah *share.Root) {
	for i, root := range dah.RowRoots {
		if !namespace.IsOutsideRange(root, root) {
			nd, err := f.Data(context.Background(), namespace, i)
			require.NoError(t, err)
			ok := nd.Verify(root, namespace)
			require.True(t, ok)
		}
	}
}

func testFileAxisHalf(t *testing.T, createFile createFile, size int) {
	eds := edstest.RandEDS(t, size)
	fl := createFile(eds)

	for _, axisType := range []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row} {
		for i := 0; i < size; i++ {
			half, err := fl.AxisHalf(context.Background(), axisType, i)
			require.NoError(t, err)
			require.Equal(t, getAxis(eds, axisType, i)[:size], half)
		}
	}
}

func testFileEds(t *testing.T, createFile createFile, size int) {
	eds := edstest.RandEDS(t, size)
	fl := createFile(eds)

	eds2, err := fl.EDS(context.Background())
	require.NoError(t, err)
	require.True(t, eds.Equals(eds2))
}

func testFileReader(t *testing.T, createFile createFile, odsSize int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eds := edstest.RandEDS(t, odsSize)
	f := createFile(eds)

	reader, err := f.Reader()
	require.NoError(t, err)

	streamed, err := ReadEds(ctx, reader, f.Size())
	require.NoError(t, err)
	require.True(t, eds.Equals(streamed))

	// verify that the reader represented by file can be read from
	// multiple times, without exhausting the underlying reader.
	reader2, err := f.Reader()
	require.NoError(t, err)

	streamed2, err := ReadEds(ctx, reader2, f.Size())
	require.NoError(t, err)
	require.True(t, eds.Equals(streamed2))
}

func benchGetAxisFromFile(b *testing.B, newFile func(size int) EdsFile, minSize, maxSize int) {
	for size := minSize; size <= maxSize; size *= 2 {
		f := newFile(size)

		// loop over all possible axis types and quadrants
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
			for _, squareHalf := range []int{0, 1} {
				name := fmt.Sprintf("Size:%v/Axis:%s/squareHalf:%s", size, axisType, strconv.Itoa(squareHalf))
				b.Run(name, func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, err := f.AxisHalf(context.TODO(), axisType, f.Size()/2*(squareHalf))
						require.NoError(b, err)
					}
				})
			}
		}
	}
}

func benchGetShareFromFile(b *testing.B, newFile func(size int) EdsFile, minSize, maxSize int) {
	for size := minSize; size <= maxSize; size *= 2 {
		f := newFile(size)

		// loop over all possible axis types and quadrants
		for _, q := range quadrants {
			name := fmt.Sprintf("Size:%v/quadrant:%s", size, q)
			b.Run(name, func(b *testing.B) {
				x, y := q.coordinates(f.Size())
				// warm up cache
				_, err := f.Share(context.TODO(), x, y)
				require.NoError(b, err)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := f.Share(context.TODO(), x, y)
					require.NoError(b, err)
				}
			})
		}

	}
}

type quadrant int

var (
	quadrants = []quadrant{1, 2, 3, 4}
)

func (q quadrant) String() string {
	return strconv.Itoa(int(q))
}

func (q quadrant) coordinates(edsSize int) (x, y int) {
	x = edsSize/2*(int(q-1)%2) + 1
	y = edsSize/2*(int(q-1)/2) + 1
	return
}
