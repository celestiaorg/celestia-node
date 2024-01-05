package store

import (
	"testing"

	"github.com/klauspost/reedsolomon"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func BenchmarkCodec(b *testing.B) {
	size := 128

	shards := make([][]byte, size)
	original := sharetest.RandShares(b, size/2)
	copy(shards, original)

	// BenchmarkLeoCodec/Leopard-10         	   81866	     14611 ns/op
	b.Run("Leopard", func(b *testing.B) {
		enc, err := reedsolomon.New(size/2, size/2, reedsolomon.WithLeopardGF(true))
		require.NoError(b, err)

		// fill with parity empty shares
		for j := len(original); j < len(shards); j++ {
			shards[j] = make([]byte, len(original[0]))
		}

		for i := 0; i < b.N; i++ {
			err = enc.Encode(shards)
			require.NoError(b, err)
		}
	})

	// BenchmarkLeoCodec/Leopard-10         	   81646	     14641 ns/op
	b.Run("default", func(b *testing.B) {
		enc, err := reedsolomon.New(size/2, size/2, reedsolomon.WithLeopardGF(false))
		require.NoError(b, err)

		// fill with parity empty shares
		for j := len(original); j < len(shards); j++ {
			shards[j] = make([]byte, len(original[0]))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err = enc.Encode(shards)
			require.NoError(b, err)
		}
	})

	// BenchmarkLeoCodec/default,_reconstructSome-10         	  407635	      2728 ns/op
	b.Run("default, reconstructSome", func(b *testing.B) {
		enc, err := reedsolomon.New(size/2, size/2, reedsolomon.WithLeopardGF(false))
		require.NoError(b, err)

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
