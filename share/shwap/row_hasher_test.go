package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/testing/edstest"
)

func TestRowHasher(t *testing.T) {
	hasher := &RowHasher{}

	_, err := hasher.Write([]byte("hello"))
	assert.Error(t, err)

	square := edstest.RandEDS(t, 2)
	row, err := newRowFromEDS(square, 1, 1)
	require.NoError(t, err)

	data, err := row.ToProto().Marshal()
	require.NoError(t, err)

	n, err := hasher.Write(data)
	require.NoError(t, err)
	assert.EqualValues(t, len(data), n)

	digest := hasher.Sum(nil)
	idBin := row.RowID.MarshalBinary()
	assert.EqualValues(t, idBin, digest)

	hasher.Reset()
	digest = hasher.Sum(nil)
	assert.NotEqualValues(t, digest, idBin)
}
