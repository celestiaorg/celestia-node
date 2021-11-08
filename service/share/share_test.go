package share

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetShare(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	serv, dah := RandServiceWithSquare(t, n)
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
		amountShares int
	}{
		{amountShares: 4}, {amountShares: 16}, {amountShares: 128},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			serv, root := RandServiceWithTree(t, tt.amountShares)
			randNID := root.RowsRoots[(len(root.RowsRoots)-1)/2][:8]

			shares, err := serv.GetSharesByNamespace(context.Background(), root, randNID)
			require.NoError(t, err)
			assert.Len(t, shares, 1)
			assert.Equal(t, randNID, []byte(shares[0].NamespaceID()))
		})
	}
}
