package eds

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// TestRsmt2DFromSharesRootsMatchPlainConstructor is the differential guard for
// the buffered-pool EDS construction: Rsmt2DFromShares (which builds via a
// pooled wrapper.TreePool) must produce byte-identical RowRoots, ColumnRoots and
// DAH hash as the plain allocating constructor, across every valid square size.
func TestRsmt2DFromSharesRootsMatchPlainConstructor(t *testing.T) {
	for _, odsSize := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512} {
		t.Run(fmt.Sprintf("ods%d", odsSize), func(t *testing.T) {
			shares, err := libshare.RandShares(odsSize * odsSize)
			require.NoError(t, err)

			// pooled path under test
			pooled, err := Rsmt2DFromShares(shares)
			require.NoError(t, err)
			pooledRoots, err := share.NewAxisRoots(pooled.ExtendedDataSquare)
			require.NoError(t, err)

			// reference: plain allocating constructor
			ref, err := rsmt2d.ComputeExtendedDataSquare(
				libshare.ToBytes(shares), share.DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(odsSize)))
			require.NoError(t, err)
			refRoots, err := share.NewAxisRoots(ref)
			require.NoError(t, err)

			require.Equal(t, refRoots.RowRoots, pooledRoots.RowRoots, "row roots differ")
			require.Equal(t, refRoots.ColumnRoots, pooledRoots.ColumnRoots, "column roots differ")
			require.True(t, bytes.Equal(refRoots.Hash(), pooledRoots.Hash()), "DAH hash differs")
		})
	}
}

// TestRsmt2DFromSharesConcurrent builds squares of mixed sizes concurrently through the shared
// pool.
func TestRsmt2DFromSharesConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 32 {
		odsSize := []int{2, 8, 32, 64}[i%4]
		shares, err := libshare.RandShares(odsSize * odsSize)
		require.NoError(t, err)

		wg.Add(1)
		go func() {
			defer wg.Done()
			pooled, err := Rsmt2DFromShares(shares)
			if !assert.NoError(t, err) {
				return
			}
			pooledRoots, err := share.NewAxisRoots(pooled.ExtendedDataSquare)
			if !assert.NoError(t, err) {
				return
			}

			ref, err := rsmt2d.ComputeExtendedDataSquare(
				libshare.ToBytes(shares), share.DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(odsSize)))
			if !assert.NoError(t, err) {
				return
			}
			refRoots, err := share.NewAxisRoots(ref)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, refRoots.RowRoots, pooledRoots.RowRoots, "row roots differ (ods%d)", odsSize)
			assert.Equal(t, refRoots.ColumnRoots, pooledRoots.ColumnRoots, "column roots differ (ods%d)", odsSize)
		}()
	}
	wg.Wait()
}

// BenchmarkRsmt2DFromShares measures the full EDS construction including root
// computation (the allocation-heavy part the pool targets), comparing the plain
// allocating constructor against the pooled Rsmt2DFromShares.
func BenchmarkRsmt2DFromShares(b *testing.B) {
	codec := share.DefaultRSMT2DCodec()
	for _, odsSize := range []int{32, 128, 256, 512} {
		shares, err := libshare.RandShares(odsSize * odsSize)
		require.NoError(b, err)

		b.Run(fmt.Sprintf("ods%d/plain", odsSize), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				eds, err := rsmt2d.ComputeExtendedDataSquare(
					libshare.ToBytes(shares), codec, wrapper.NewConstructor(uint64(odsSize)))
				require.NoError(b, err)
				_, err = share.NewAxisRoots(eds)
				require.NoError(b, err)
			}
		})
		b.Run(fmt.Sprintf("ods%d/pooled", odsSize), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				eds, err := Rsmt2DFromShares(shares)
				require.NoError(b, err)
				_, err = share.NewAxisRoots(eds.ExtendedDataSquare)
				require.NoError(b, err)
			}
		})
	}
}
