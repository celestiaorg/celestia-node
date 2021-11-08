package share

import (
	"context"
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

// TODO @renaynay: make this table test w/ overflowing shares
func TestService_GetSharesByNamespace(t *testing.T) {
	serv, root := RandServiceWithTree(t, 16)
	randRow := root.RowsRoots[len(root.RowsRoots)/2]
	randNID := randRow[:8]

	shares, err := serv.GetSharesByNamespace(context.Background(), root, randNID)
	require.NoError(t, err)

	assert.True(t, len(shares) == 1)
	assert.Equal(t, randRow[8:], shares[0][8:])
}
