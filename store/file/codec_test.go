package file

import (
	"fmt"
	"testing"

	"github.com/klauspost/reedsolomon"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"
)

func BenchmarkCodec(b *testing.B) {
	minSize, maxSize := 32, 128

	for size := minSize; size <= maxSize; size *= 2 {
		// BenchmarkCodec/Leopard/size:32-10         					  409194	      2793 ns/op
		// BenchmarkCodec/Leopard/size:64-10                         	  190969	      6170 ns/op
		// BenchmarkCodec/Leopard/size:128-10                        	   82821	     14287 ns/op
		b.Run(fmt.Sprintf("Leopard/size:%v", size), func(b *testing.B) {
			enc, err := reedsolomon.New(size/2, size/2, reedsolomon.WithLeopardGF(true))
			require.NoError(b, err)

			shards := newShards(b, size, true)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err = enc.Encode(shards)
				require.NoError(b, err)
			}
		})

		// BenchmarkCodec/default/size:32-10       					  	  222153	      5364 ns/op
		// BenchmarkCodec/default/size:64-10                         	   58831	     20349 ns/op
		// BenchmarkCodec/default/size:128-10                        	   14940	     80471 ns/op
		b.Run(fmt.Sprintf("default/size:%v", size), func(b *testing.B) {
			enc, err := reedsolomon.New(size/2, size/2, reedsolomon.WithLeopardGF(false))
			require.NoError(b, err)

			shards := newShards(b, size, true)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err = enc.Encode(shards)
				require.NoError(b, err)
			}
		})

		// BenchmarkCodec/default-reconstructSome/size:32-10         	 1263585	       954.4 ns/op
		// BenchmarkCodec/default-reconstructSome/size:64-10         	  762273	      1554 ns/op
		// BenchmarkCodec/default-reconstructSome/size:128-10        	  429268	      2974 ns/op
		b.Run(fmt.Sprintf("default-reconstructSome/size:%v", size), func(b *testing.B) {
			enc, err := reedsolomon.New(size/2, size/2, reedsolomon.WithLeopardGF(false))
			require.NoError(b, err)

			shards := newShards(b, size, false)
			targets := make([]bool, size)
			target := size - 2
			targets[target] = true

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err = enc.ReconstructSome(shards, targets)
				require.NoError(b, err)
				shards[target] = nil
			}
		})
	}
}

func newShards(b testing.TB, size int, fillParity bool) [][]byte {
	shards := make([][]byte, size)
	original, err := libshare.RandShares(size / 2)
	require.NoError(b, err)
	copy(shards, libshare.ToBytes(original))

	if fillParity {
		// fill with parity empty Shares
		for j := len(original); j < len(shards); j++ {
			shards[j] = make([]byte, libshare.ShareSize)
		}
	}
	return shards
}
