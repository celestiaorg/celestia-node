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
	serv, root := RandServiceWithTree(t, 4)
	randNID := root.RowsRoots[len(root.RowsRoots)/2][8:16]
	t.Log("RAND NID: ", randNID)

	shares, err := serv.GetSharesByNamespace(context.Background(), root, randNID)
	require.NoError(t, err)

	t.Log("len shares: ", len(shares))

	for _, share := range shares {
		t.Log("len of share: ", len(share))
		t.Log("share: ", share)
	}

	assert.Equal(t, randNID, []byte(shares[0].NamespaceID()))
}
