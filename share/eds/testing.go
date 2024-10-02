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
		for name, eds := range testEDSes(t, size) {
			t.Run(fmt.Sprintf("DataHash:%s", name), func(t *testing.T) {
				t.Parallel()
				testAccessorDataHash(ctx, t, createAccessor, eds)
			})

			t.Run(fmt.Sprintf("AxisRoots:%s", name), func(t *testing.T) {
				t.Parallel()
				testAccessorAxisRoots(ctx, t, createAccessor, eds)
			})

			t.Run(fmt.Sprintf("Sample:%s", name), func(t *testing.T) {
				t.Parallel()
				testAccessorSample(ctx, t, createAccessor, eds)
			})

			t.Run(fmt.Sprintf("AxisHalf:%s", name), func(t *testing.T) {
				t.Parallel()
				testAccessorAxisHalf(ctx, t, createAccessor, eds)
			})

			t.Run(fmt.Sprintf("RowNamespaceData:%s", name), func(t *testing.T) {
				t.Parallel()
				testAccessorRowNamespaceData(ctx, t, createAccessor, size)
			})

			t.Run(fmt.Sprintf("Shares:%s", name), func(t *testing.T) {
				t.Parallel()
				testAccessorShares(ctx, t, createAccessor, eds)
			})
		}
	}
}

func testEDSes(t *testing.T, sizes ...int) map[string]*rsmt2d.ExtendedDataSquare {
	testEDSes := make(map[string]*rsmt2d.ExtendedDataSquare)
	for _, size := range sizes {
		fullEDS := edstest.RandEDS(t, size)
		testEDSes[fmt.Sprintf("FullODS:%d", size)] = fullEDS

		var padding int
		for padding < 1 {
			padding = rand.IntN(size * size) //nolint:gosec
		}
		paddingEds := edstest.RandEDSWithTailPadding(t, size, padding)
		testEDSes[fmt.Sprintf("PaddedODS:%d", size)] = paddingEds
	}

	testEDSes["EmptyODS"] = share.EmptyEDS()
	return testEDSes
}

func TestStreamer(
	ctx context.Context,
	t *testing.T,
	create createAccessorStreamer,
	odsSize int,
) {
	for name, eds := range testEDSes(t, odsSize) {
		t.Run(fmt.Sprintf("Reader:%s", name), func(t *testing.T) {
			t.Parallel()
			testAccessorReader(ctx, t, create, eds)
		})
	}
}

func testAccessorDataHash(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	eds *rsmt2d.ExtendedDataSquare,
) {
	acc := createAccessor(t, eds)

	expected, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	datahash, err := acc.DataHash(ctx)
	require.NoError(t, err)
	require.Equal(t, share.DataHash(expected.Hash()), datahash)
}

func testAccessorAxisRoots(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	eds *rsmt2d.ExtendedDataSquare,
) {
	acc := createAccessor(t, eds)

	expected, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	roots, err := acc.AxisRoots(ctx)
	require.NoError(t, err)
	require.True(t, expected.Equals(roots))
}

func testAccessorSample(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	eds *rsmt2d.ExtendedDataSquare,
) {
	width := int(eds.Width())
	t.Run("single thread", func(t *testing.T) {
		acc := createAccessor(t, eds)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		// t.Parallel() this fails the test for some reason
		for rowIdx := 0; rowIdx < width; rowIdx++ {
			for colIdx := 0; colIdx < width; colIdx++ {
				testSample(ctx, t, acc, roots, colIdx, rowIdx)
			}
		}
	})

	t.Run("parallel", func(t *testing.T) {
		t.Parallel()
		acc := createAccessor(t, eds)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		wg := sync.WaitGroup{}
		for rowIdx := 0; rowIdx < width; rowIdx++ {
			for colIdx := 0; colIdx < width; colIdx++ {
				wg.Add(1)
				go func(rowIdx, colIdx int) {
					defer wg.Done()
					testSample(ctx, t, acc, roots, rowIdx, colIdx)
				}(rowIdx, colIdx)
			}
		}
		wg.Wait()
	})

	t.Run("random", func(t *testing.T) {
		t.Parallel()
		acc := createAccessor(t, eds)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)

		wg := sync.WaitGroup{}
		for range 1000 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rowIdx, colIdx := rand.IntN(width), rand.IntN(width) //nolint:gosec
				testSample(ctx, t, acc, roots, rowIdx, colIdx)
			}()
		}
		wg.Wait()
	})
}

func testSample(
	ctx context.Context,
	t *testing.T,
	acc Accessor,
	roots *share.AxisRoots,
	rowIdx, colIdx int,
) {
	shr, err := acc.Sample(ctx, rowIdx, colIdx)
	require.NoError(t, err)

	err = shr.Verify(roots, rowIdx, colIdx)
	require.NoError(t, err)
}

func testAccessorRowNamespaceData(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessor,
	odsSize int,
) {
	t.Run("included", func(t *testing.T) {
		t.Parallel()
		// generate EDS with random data and some Shares with the same namespace
		sharesAmount := odsSize * odsSize
		namespace := sharetest.RandV0Namespace()
		// test with different amount of shares
		for amount := 1; amount < sharesAmount; amount++ {
			// select random amount of shares, but not less than 1
			eds, roots := edstest.RandEDSWithNamespace(t, namespace, amount, odsSize)
			acc := createAccessor(t, eds)

			var actualSharesAmount int
			// loop over all rows and check that the amount of shares in the namespace is equal to the expected
			// amount
			for i, root := range roots.RowRoots {
				rowData, err := acc.RowNamespaceData(ctx, namespace, i)

				// namespace is not included in the row, so there should be no shares
				if namespace.IsOutsideRange(root, root) {
					require.ErrorIs(t, err, shwap.ErrNamespaceOutsideRange)
					require.Len(t, rowData.Shares, 0)
					continue
				}

				actualSharesAmount += len(rowData.Shares)
				require.NoError(t, err)
				require.True(t, len(rowData.Shares) > 0)
				err = rowData.Verify(roots, namespace, i)
				require.NoError(t, err)
			}

			// check that the amount of shares in the namespace is equal to the expected amount
			require.Equal(t, amount, actualSharesAmount)
		}
	})

	t.Run("not included", func(t *testing.T) {
		t.Parallel()
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

			acc := createAccessor(t, eds)
			rowData, err := acc.RowNamespaceData(ctx, absentNs, i)
			require.NoError(t, err)

			// namespace is not included in the row, so there should be no shares
			require.Len(t, rowData.Shares, 0)
			require.True(t, rowData.Proof.IsOfAbsence())

			err = rowData.Verify(roots, absentNs, i)
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
	acc := createAccessor(t, eds)

	t.Run("single thread", func(t *testing.T) {
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row} {
			for axisIdx := 0; axisIdx < int(eds.Width()); axisIdx++ {
				half, err := acc.AxisHalf(ctx, axisType, axisIdx)
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
		t.Parallel()
		wg := sync.WaitGroup{}
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row} {
			for i := 0; i < int(eds.Width()); i++ {
				wg.Add(1)
				go func(axisType rsmt2d.Axis, idx int) {
					defer wg.Done()
					half, err := acc.AxisHalf(ctx, axisType, idx)
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
	acc := createAccessor(t, eds)

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			shares, err := acc.Shares(ctx)
			require.NoError(t, err)
			expected := eds.FlattenedODS()
			require.Equal(t, expected, shares)
		}()
	}
	wg.Wait()
}

func testAccessorReader(
	ctx context.Context,
	t *testing.T,
	createAccessor createAccessorStreamer,
	eds *rsmt2d.ExtendedDataSquare,
) {
	acc := createAccessor(t, eds)

	// verify that the reader represented by accessor can be read from
	// multiple times, without exhausting the underlying reader.
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testReader(ctx, t, eds, acc)
		}()
	}
	wg.Wait()
}

func testReader(ctx context.Context, t *testing.T, eds *rsmt2d.ExtendedDataSquare, as AccessorStreamer) {
	reader, err := as.Reader()
	require.NoError(t, err)

	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	actual, err := ReadAccessor(ctx, reader, roots)
	require.NoError(t, err)
	require.True(t, eds.Equals(actual.ExtendedDataSquare))
}

func BenchGetHalfAxisFromAccessor(
	ctx context.Context,
	b *testing.B,
	createAccessor createAccessor,
	minOdsSize, maxOdsSize int,
) {
	for size := minOdsSize; size <= maxOdsSize; size *= 2 {
		eds := edstest.RandEDS(b, size)
		acc := createAccessor(b, eds)

		// loop over all possible axis types and quadrants
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
			for _, squareHalf := range []int{0, 1} {
				name := fmt.Sprintf("Size:%v/ProofType:%s/squareHalf:%s", size, axisType, strconv.Itoa(squareHalf))
				b.Run(name, func(b *testing.B) {
					// warm up cache
					_, err := acc.AxisHalf(ctx, axisType, acc.Size(ctx)/2*(squareHalf))
					require.NoError(b, err)

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, err := acc.AxisHalf(ctx, axisType, acc.Size(ctx)/2*(squareHalf))
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
	createAccessor createAccessor,
	minOdsSize, maxOdsSize int,
) {
	for size := minOdsSize; size <= maxOdsSize; size *= 2 {
		eds := edstest.RandEDS(b, size)
		acc := createAccessor(b, eds)

		// loop over all possible axis types and quadrants
		for _, q := range quadrants {
			name := fmt.Sprintf("Size:%v/quadrant:%s", size, q)
			b.Run(name, func(b *testing.B) {
				rowIdx, colIdx := q.coordinates(acc.Size(ctx))
				// warm up cache
				_, err := acc.Sample(ctx, rowIdx, colIdx)
				require.NoError(b, err, q.String())

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := acc.Sample(ctx, rowIdx, colIdx)
					require.NoError(b, err)
				}
			})
		}
	}
}

type quadrantIdx int

var quadrants = []quadrantIdx{1, 2, 3, 4}

func (q quadrantIdx) String() string {
	return strconv.Itoa(int(q))
}

func (q quadrantIdx) coordinates(edsSize int) (rowIdx, colIdx int) {
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
