package shwap

import (
	"github.com/celestiaorg/celestia-node/share"
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

	data, err := share.NewNamespacedSharesFromEDS(square, namespace)
	require.NoError(t, err)
	rowData := data[0]
	id, err := NewDataID(1, 1, namespace, root)
	require.NoError(t, err)

	dat, err := rowData.ToProto().Marshal()
	require.NoError(t, err)

	n, err := hasher.Write(dat)
	require.NoError(t, err)
	assert.EqualValues(t, len(dat), n)

	digest := hasher.Sum(nil)
	idBin := id.MarshalBinary()
	assert.EqualValues(t, idBin, digest)

	hasher.Reset()
	digest = hasher.Sum(nil)
	assert.NotEqualValues(t, digest, id)
}
