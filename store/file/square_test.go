package file

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"
)

func TestComputeAxisHalf(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })

	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "FF8", size: 8},
		{name: "FF16", size: 256},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := randomSquare(t, tc.size)
			for _, axis := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
				t.Run(axis.String(), func(t *testing.T) {
					expected := encodedAxisHalf(t, s, axis, tc.size)

					half, err := s.computeAxisHalf(context.Background(), axis, tc.size)
					require.NoError(t, err)
					require.False(t, half.IsParity)
					require.Equal(t, expected, libshare.ToBytes(half.Shares))
				})
			}
		})
	}
}

func TestComputeAxisHalfCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := randomSquare(t, 2).computeAxisHalf(ctx, rsmt2d.Row, 2)
	require.ErrorIs(t, err, context.Canceled)
}

func randomSquare(t *testing.T, size int) square {
	t.Helper()
	shares, err := libshare.RandShares(size * size)
	require.NoError(t, err)

	s := make(square, size)
	for i := range s {
		s[i] = shares[i*size : (i+1)*size]
	}
	return s
}

func encodedAxisHalf(t *testing.T, s square, axisType rsmt2d.Axis, axisIdx int) [][]byte {
	t.Helper()

	enc, err := codec.Encoder(s.size() * 2)
	require.NoError(t, err)

	shares := make([]libshare.Share, s.size())
	for i := range s.size() {
		half, err := s.axisHalf(oppositeAxis(axisType), i)
		require.NoError(t, err)

		shards := make([][]byte, s.size()*2)
		copy(shards, libshare.ToBytes(half.Shares))
		for i := s.size(); i < len(shards); i++ {
			shards[i] = make([]byte, libshare.ShareSize)
		}
		require.NoError(t, enc.Encode(shards))

		shares[i], err = libshare.NewShare(shards[axisIdx])
		require.NoError(t, err)
	}
	return libshare.ToBytes(shares)
}
