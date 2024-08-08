package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestRowHasher(t *testing.T) {
	hasher := &RowHasher{}

	_, err := hasher.Write([]byte("hello"))
	assert.Error(t, err)

	square := edstest.RandEDS(t, 2)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	row, err := NewRowFromEDS(2, 1, square)
	require.NoError(t, err)

	rowVerifiers.Add(row.RowID, func(row Row) error {
		return row.Verify(root)
	})

	data, err := row.MarshalBinary()
	require.NoError(t, err)

	n, err := hasher.Write(data)
	require.NoError(t, err)
	assert.EqualValues(t, len(data), n)

	digest := hasher.Sum(nil)
	id, err := row.RowID.MarshalBinary()
	require.NoError(t, err)
	assert.EqualValues(t, id, digest)

	hasher.Reset()
	digest = hasher.Sum(nil)
	assert.NotEqualValues(t, digest, id)
}
