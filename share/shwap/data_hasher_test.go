package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/testing/edstest"
	"github.com/celestiaorg/celestia-node/share/testing/sharetest"
)

func TestDataHasher(t *testing.T) {
	hasher := &DataHasher{}

	_, err := hasher.Write([]byte("hello"))
	assert.Error(t, err)

	size := 8
	namespace := sharetest.RandV0Namespace()
	square, root := edstest.RandEDSWithNamespace(t, namespace, size*size, size)

	datas, err := NewDataFromEDS(square, 1, namespace)
	require.NoError(t, err)
	data := datas[0]

	globalRootsCache.Store(data.DataID, root)

	dat, err := data.MarshalBinary()
	require.NoError(t, err)

	n, err := hasher.Write(dat)
	require.NoError(t, err)
	assert.EqualValues(t, len(dat), n)

	digest := hasher.Sum(nil)
	id := data.DataID.MarshalBinary()
	assert.EqualValues(t, id, digest)

	hasher.Reset()
	digest = hasher.Sum(nil)
	assert.NotEqualValues(t, digest, id)
}
