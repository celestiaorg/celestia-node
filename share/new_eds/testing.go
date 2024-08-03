package eds

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

type (
	createAccessor         func(testing.TB, *rsmt2d.ExtendedDataSquare) Accessor
	createAccessorStreamer func(testing.TB, *rsmt2d.ExtendedDataSquare) AccessorStreamer
)

// TestSuiteAccessor runs a suite of tests for the given Accessor implementation.
func TestSuiteAccessor(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	maxSize int,
) {
	minSize := 2
	if !checkPowerOfTwo(maxSize) {
		t.Errorf("minSize must be power of 2: %v", maxSize)
	}
	for size := minSize; size <= maxSize; size *= 2 {
		padding := rand.IntN(size)
		eds := edstest.RandEDSWithTailPadding(t, size, padding)

		t.Run(fmt.Sprintf("DataHash:%d", size), func(t *testing.T) {
			testAccessorDataHash(ctx, t, createAccessor, eds)
		})

		t.Run(fmt.Sprintf("AxisRoots:%d", size), func(t *testing.T) {
			testAccessorAxisRoots(ctx, t, createAccessor, eds)
		})

		t.Run(fmt.Sprintf("Sample:%d", size), func(t *testing.T) {
			testAccessorSample(ctx, t, createAccessor, eds)
		})

		t.Run(fmt.Sprintf("AxisHalf:%d", size), func(t *testing.T) {
			testAccessorAxisHalf(ctx, t, createAccessor, eds)
		})

		t.Run(fmt.Sprintf("RowNamespaceData:%d", size), func(t *testing.T) {
			testAccessorRowNamespaceData(ctx, t, createAccessor, size)
		})

		t.Run(fmt.Sprintf("Shares:%d", size), func(t *testing.T) {
			testAccessorShares(ctx, t, createAccessor, eds)
		})
	}
}

func TestStreamer(
	ctx context.Context,
	t *testing.T,
	create createAccessorStreamer,
	odsSize int,
) {
	padding := rand.IntN(odsSize)
	eds := edstest.RandEDSWithTailPadding(t, odsSize, padding)

	t.Run("ReaderConcurrent", func(t *testing.T) {
		testAccessorReader(ctx, t, create, eds)
	})
	t.Run("ReaderPadding", func(t *testing.T) {
		testAccessorReaderPadding(ctx, t, create, eds)
	})
}

func testAccessorDataHash(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	eds *rsmt2d.ExtendedDataSquare,
) {
	fl := createAccessor(t, eds)

	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	datahash, err := fl.DataHash(ctx)
	require.NoError(t, err)
	require.Equal(t, share.DataHash(roots.Hash()), datahash)
}

func testAccessorAxisRoots(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	eds *rsmt2d.ExtendedDataSquare,
) {
	fl := createAccessor(t, eds)

	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	axisRoots, err := fl.AxisRoots(ctx)
	require.NoError(t, err)
	require.True(t, roots.Equals(axisRoots))
}

func testAccessorSample(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	eds *rsmt2d.ExtendedDataSquare,
) {
	fl := createAccessor(t, eds)

	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	width := int(eds.Width())
	t.Run("single thread", func(t *testing.T) {
		for rowIdx := 0; rowIdx < width; rowIdx++ {
			for colIdx := 0; colIdx < width; colIdx++ {
				testSample(ctx, t, fl, roots, colIdx, rowIdx)
			}
		}
	})

	t.Run("parallel", func(t *testing.T) {
		wg := sync.WaitGroup{}
		for rowIdx := 0; rowIdx < width; rowIdx++ {
			for colIdx := 0; colIdx < width; colIdx++ {
				wg.Add(1)
				go func(rowIdx, colIdx int) {
					defer wg.Done()
					testSample(ctx, t, fl, roots, rowIdx, colIdx)
				}(rowIdx, colIdx)
			}
		}
		wg.Wait()
	})
}

func testSample(
	ctx context.Context,
	t *testing.T,
	fl Accessor,
	roots *share.AxisRoots,
	rowIdx, colIdx int,
) {
	shr, err := fl.Sample(ctx, rowIdx, colIdx)
	require.NoError(t, err)

	err = shr.Validate(roots, rowIdx, colIdx)
	require.NoError(t, err)
}

func testAccessorRowNamespaceData(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	odsSize int,
) {
	t.Run("included", func(t *testing.T) {
		// generate EDS with random data and some Shares with the same namespace
		sharesAmount := odsSize * odsSize
		namespace := sharetest.RandV0Namespace()
		// test with different amount of shares
		for amount := 1; amount < sharesAmount; amount++ {
			// select random amount of shares, but not less than 1
			eds, roots := edstest.RandEDSWithNamespace(t, namespace, amount, odsSize)
			f := createAccessor(t, eds)

			var actualSharesAmount int
			// loop over all rows and check that the amount of shares in the namespace is equal to the expected
			// amount
			for i, root := range roots.RowRoots {
				rowData, err := f.RowNamespaceData(ctx, namespace, i)

				// namespace is not included in the row, so there should be no shares
				if namespace.IsOutsideRange(root, root) {
					require.ErrorIs(t, err, shwap.ErrNamespaceOutsideRange)
					require.Len(t, rowData.Shares, 0)
					continue
				}

				actualSharesAmount += len(rowData.Shares)
				require.NoError(t, err)
				require.True(t, len(rowData.Shares) > 0)
				err = rowData.Validate(roots, namespace, i)
				require.NoError(t, err)
			}

			// check that the amount of shares in the namespace is equal to the expected amount
			require.Equal(t, amount, actualSharesAmount)
		}
	})

	t.Run("not included", func(t *testing.T) {
		// generate EDS with random data and some Shares with the same namespace
		eds := edstest.RandEDS(t, odsSize)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)

		// loop over first half of the rows, because the second half is parity and does not contain
		// namespaced shares
		for i, root := range roots.RowRoots[:odsSize] {
			// select namespace that within the range of root namespaces, but is not included
			maxNs := nmt.MaxNamespace(root, share.NamespaceSize)
			absentNs, err := share.Namespace(maxNs).AddInt(-1)
			require.NoError(t, err)

			f := createAccessor(t, eds)
			rowData, err := f.RowNamespaceData(ctx, absentNs, i)
			require.NoError(t, err)

			// namespace is not included in the row, so there should be no shares
			require.Len(t, rowData.Shares, 0)
			require.True(t, rowData.Proof.IsOfAbsence())

			err = rowData.Validate(roots, absentNs, i)
			require.NoError(t, err)
		}
	})
}

func testAccessorAxisHalf(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	eds *rsmt2d.ExtendedDataSquare,
) {
	odsSize := int(eds.Width() / 2)
	fl := createAccessor(t, eds)

	t.Run("single thread", func(t *testing.T) {
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row} {
			for axisIdx := 0; axisIdx < int(eds.Width()); axisIdx++ {
				half, err := fl.AxisHalf(ctx, axisType, axisIdx)
				require.NoError(t, err)
				require.Len(t, half.Shares, odsSize)

				var expected []share.Share
				if half.IsParity {
					expected = getAxis(eds, axisType, axisIdx)[odsSize:]
				} else {
					expected = getAxis(eds, axisType, axisIdx)[:odsSize]
				}

				require.Equal(t, expected, half.Shares)
			}
		}
	})

	t.Run("parallel", func(t *testing.T) {
		wg := sync.WaitGroup{}
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row} {
			for i := 0; i < int(eds.Width()); i++ {
				wg.Add(1)
				go func(axisType rsmt2d.Axis, idx int) {
					defer wg.Done()
					half, err := fl.AxisHalf(ctx, axisType, idx)
					require.NoError(t, err)
					require.Len(t, half.Shares, odsSize)

					var expected []share.Share
					if half.IsParity {
						expected = getAxis(eds, axisType, idx)[odsSize:]
					} else {
						expected = getAxis(eds, axisType, idx)[:odsSize]
					}

					require.Equal(t, expected, half.Shares)
				}(axisType, i)
			}
		}
		wg.Wait()
	})
}

func testAccessorShares(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	eds *rsmt2d.ExtendedDataSquare,
) {
	fl := createAccessor(t, eds)

	shares, err := fl.Shares(ctx)
	require.NoError(t, err)
	expected := eds.FlattenedODS()
	require.Equal(t, expected, shares)
}

func testAccessorReaderPadding(
	ctx context.Context,
	t *testing.T,
	create createAccessorStreamer,
	eds *rsmt2d.ExtendedDataSquare,
) {
	f := create(t, eds)
	testReader(ctx, t, eds, f)
}

func testAccessorReader(
	ctx context.Context,
	t *testing.T,
	create createAccessorStreamer,
	eds *rsmt2d.ExtendedDataSquare,
) {
	f := create(t, eds)

	// verify that the reader represented by file can be read from
	// multiple times, without exhausting the underlying reader.
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testReader(ctx, t, eds, f)
		}()
	}
	wg.Wait()
}

func testReader(ctx context.Context, t *testing.T, eds *rsmt2d.ExtendedDataSquare, as AccessorStreamer) {
	reader, err := as.Reader()
	require.NoError(t, err)

	roots, err := as.AxisRoots(ctx)
	require.NoError(t, err)

	actual, err := ReadEDS(ctx, reader, roots)
	require.NoError(t, err)
	require.True(t, eds.Equals(actual))
}

func BenchGetHalfAxisFromAccessor(
	ctx context.Context,
	b *testing.B,
	newAccessor func(size int) Accessor,
	minOdsSize, maxOdsSize int,
) {
	for size := minOdsSize; size <= maxOdsSize; size *= 2 {
		f := newAccessor(size)

		// loop over all possible axis types and quadrants
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
			for _, squareHalf := range []int{0, 1} {
				name := fmt.Sprintf("Size:%v/ProofType:%s/squareHalf:%s", size, axisType, strconv.Itoa(squareHalf))
				b.Run(name, func(b *testing.B) {
					// warm up cache
					_, err := f.AxisHalf(ctx, axisType, f.Size(ctx)/2*(squareHalf))
					require.NoError(b, err)

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, err := f.AxisHalf(ctx, axisType, f.Size(ctx)/2*(squareHalf))
						require.NoError(b, err)
					}
				})
			}
		}
	}
}

func BenchGetSampleFromAccessor(
	ctx context.Context,
	b *testing.B,
	newAccessor func(size int) Accessor,
	minOdsSize, maxOdsSize int,
) {
	for size := minOdsSize; size <= maxOdsSize; size *= 2 {
		f := newAccessor(size)

		// loop over all possible axis types and quadrants
		for _, q := range quadrants {
			name := fmt.Sprintf("Size:%v/quadrant:%s", size, q)
			b.Run(name, func(b *testing.B) {
				rowIdx, colIdx := q.coordinates(f.Size(ctx))
				// warm up cache
				_, err := f.Sample(ctx, rowIdx, colIdx)
				require.NoError(b, err, q.String())

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := f.Sample(ctx, rowIdx, colIdx)
					require.NoError(b, err)
				}
			})
		}
	}
}

type quadrant int

var quadrants = []quadrant{1, 2, 3, 4}

func (q quadrant) String() string {
	return strconv.Itoa(int(q))
}

func (q quadrant) coordinates(edsSize int) (rowIdx, colIdx int) {
	colIdx = edsSize/2*(int(q-1)%2) + 1
	rowIdx = edsSize/2*(int(q-1)/2) + 1
	return rowIdx, colIdx
}

func checkPowerOfTwo(n int) bool {
	// added one corner case if n is zero it will also consider as power 2
	if n == 0 {
		return true
	}
	return n&(n-1) == 0
}
