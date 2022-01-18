package share

import (
	"context"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/pkg/da"
)

func TestGetShare(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	serv, dah := RandLightServiceWithSquare(t, n)
	err := serv.Start(ctx)
	require.NoError(t, err)

	for i := range make([]bool, n) {
		for j := range make([]bool, n) {
			share, err := serv.GetShare(ctx, dah, i, j)
			assert.NotNil(t, share)
			assert.NoError(t, err)
		}
	}

	err = serv.Stop(ctx)
	require.NoError(t, err)
}

func TestService_GetSharesByNamespace(t *testing.T) {
	var tests = []struct {
		amountShares       int
		expectedShareCount int
	}{
		{amountShares: 4, expectedShareCount: 1},
		{amountShares: 16, expectedShareCount: 2},
		{amountShares: 128, expectedShareCount: 1},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			serv, root := RandLightServiceWithSquare(t, tt.amountShares)
			randNID := root.RowsRoots[(len(root.RowsRoots)-1)/2][:8]
			if tt.expectedShareCount > 1 {
				// make it so that two rows have the same namespace ID
				root.RowsRoots[(len(root.RowsRoots) / 2)] = root.RowsRoots[(len(root.RowsRoots)-1)/2]
			}

			shares, err := serv.GetSharesByNamespace(context.Background(), root, randNID)
			require.NoError(t, err)
			assert.Len(t, shares, tt.expectedShareCount)
			for _, value := range shares {
				assert.Equal(t, randNID, []byte(value.NamespaceID()))
			}
		})
	}
}

func TestGetShares(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	serv, dah := RandLightServiceWithSquare(t, n)
	err := serv.Start(ctx)
	require.NoError(t, err)

	shares, err := serv.GetShares(ctx, dah)
	require.NoError(t, err)

	flattened := make([][]byte, 0)
	for _, row := range shares {
		for _, share := range row {
			flattened = append(flattened, share)
		}
	}
	// generate DAH from shares returned by `GetShares` to compare
	// calculated DAH to expected DAH
	squareSize := uint64(math.Sqrt(float64(len(flattened))))
	eds, err := da.ExtendShares(squareSize, flattened)
	require.NoError(t, err)
	gotDAH := da.NewDataAvailabilityHeader(eds)

	require.True(t, dah.Equals(&gotDAH))

	err = serv.Stop(ctx)
	require.NoError(t, err)
}

func TestService_GetSharesByNamespaceNotFoundInRange(t *testing.T) {
	serv, root := RandLightServiceWithSquare(t, 1)
	root.RowsRoots = nil

	shares, err := serv.GetSharesByNamespace(context.Background(), root, []byte(""))
	assert.Len(t, shares, 0)
	require.Error(t, err, "namespaceID not found in range")
}

func BenchmarkService_GetSharesByNamespace(b *testing.B) {
	var tests = []struct {
		amountShares int
	}{
		{amountShares: 4},
		{amountShares: 16},
		{amountShares: 128},
	}

	for _, tt := range tests {
		b.Run(strconv.Itoa(tt.amountShares), func(b *testing.B) {
			t := &testing.T{}
			serv, root := RandLightServiceWithSquare(t, tt.amountShares)
			randNID := root.RowsRoots[(len(root.RowsRoots)-1)/2][:8]
			root.RowsRoots[(len(root.RowsRoots) / 2)] = root.RowsRoots[(len(root.RowsRoots)-1)/2]
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := serv.GetSharesByNamespace(context.Background(), root, randNID)
				require.NoError(t, err)
			}
		})
	}
}
