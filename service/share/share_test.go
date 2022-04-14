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
		squareSize         int
		expectedShareCount int
	}{
		{squareSize: 4, expectedShareCount: 1},
		{squareSize: 16, expectedShareCount: 2},
		{squareSize: 128, expectedShareCount: 1},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			serv, dag := RandLightService()
			n := tt.squareSize * tt.squareSize
			randShares := RandShares(t, n)
			idx1 := (n - 1) / 2
			idx2 := n / 2
			if tt.expectedShareCount > 1 {
				// make it so that two rows have the same namespace ID
				copy(randShares[idx2][:8], randShares[idx1][:8])
			}
			root := FillDag(t, tt.squareSize, dag, randShares)
			randNID := []byte(randShares[idx1][:8])

			shares, err := serv.GetSharesByNamespace(context.Background(), root, randNID)
			require.NoError(t, err)
			assert.Len(t, shares, tt.expectedShareCount)
			for _, value := range shares {
				assert.Equal(t, randNID, []byte(value.NamespaceID()))
			}
			if tt.expectedShareCount > 1 {
				// idx1 is always smaller than idx2
				assert.Equal(t, []byte(randShares[idx1]), shares[0].Data())
				assert.Equal(t, []byte(randShares[idx2]), shares[1].Data())
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
