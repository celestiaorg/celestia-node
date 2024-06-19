package eds

import (
	"context"
	"fmt"
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

type createAccessor func(testing.TB, *rsmt2d.ExtendedDataSquare) Accessor

// TestSuiteAccessor runs a suite of tests for the given Accessor implementation.
func TestSuiteAccessor(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	odsSize int,
) {
	t.Run("Sample", func(t *testing.T) {
		testAccessorSample(ctx, t, createAccessor, odsSize)
	})

	t.Run("AxisHalf", func(t *testing.T) {
		testAccessorAxisHalf(ctx, t, createAccessor, odsSize)
	})

	t.Run("RowNamespaceData", func(t *testing.T) {
		testAccessorRowNamespaceData(ctx, t, createAccessor, odsSize)
	})

	t.Run("Shares", func(t *testing.T) {
		testAccessorShares(ctx, t, createAccessor, odsSize)
	})
}

func testAccessorSample(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	odsSize int,
) {
	eds := edstest.RandEDS(t, odsSize)
	fl := createAccessor(t, eds)

	dah, err := share.NewRoot(eds)
	require.NoError(t, err)

	width := int(eds.Width())
	t.Run("single thread", func(t *testing.T) {
		for rowIdx := 0; rowIdx < width; rowIdx++ {
			for colIdx := 0; colIdx < width; colIdx++ {
				testSample(ctx, t, fl, dah, colIdx, rowIdx)
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
					testSample(ctx, t, fl, dah, rowIdx, colIdx)
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
	dah *share.Root,
	rowIdx, colIdx int,
) {
	shr, err := fl.Sample(ctx, rowIdx, colIdx)
	require.NoError(t, err)

	err = shr.Validate(dah, rowIdx, colIdx)
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
			eds, dah := edstest.RandEDSWithNamespace(t, namespace, amount, odsSize)
			f := createAccessor(t, eds)

			var actualSharesAmount int
			// loop over all rows and check that the amount of shares in the namespace is equal to the expected
			// amount
			for i, root := range dah.RowRoots {
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
				err = rowData.Validate(dah, namespace, i)
				require.NoError(t, err)
			}

			// check that the amount of shares in the namespace is equal to the expected amount
			require.Equal(t, amount, actualSharesAmount)
		}
	})

	t.Run("not included", func(t *testing.T) {
		// generate EDS with random data and some Shares with the same namespace
		eds := edstest.RandEDS(t, odsSize)
		dah, err := share.NewRoot(eds)
		require.NoError(t, err)

		// loop over first half of the rows, because the second half is parity and does not contain
		// namespaced shares
		for i, root := range dah.RowRoots[:odsSize] {
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

			err = rowData.Validate(dah, absentNs, i)
			require.NoError(t, err)
		}
	})
}

func testAccessorAxisHalf(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	odsSize int,
) {
	eds := edstest.RandEDS(t, odsSize)
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
	odsSize int,
) {
	eds := edstest.RandEDS(t, odsSize)
	fl := createAccessor(t, eds)

	shares, err := fl.Shares(ctx)
	require.NoError(t, err)
	expected := eds.FlattenedODS()
	require.Equal(t, expected, shares)
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
		for _, squareHalf := range []int{0, 1} {
			for _, axisType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
				name := fmt.Sprintf("Size:%v/ProofType:%s/squareHalf:%s", size, axisType, strconv.Itoa(squareHalf))
				b.Run(name, func(b *testing.B) {
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
