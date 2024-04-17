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

	datas, err := newDataFromEDS(square, 1, namespace)
	data := datas[0]

	err = data.Validate(root, int(data.DataID.RowIndex), namespace)
	require.NoError(t, err)

	bin, err := data.ToProto().Marshal()
	require.NoError(t, err)

	n, err := hasher.Write(bin)
	require.NoError(t, err)
	assert.EqualValues(t, len(bin), n)

	digest := hasher.Sum(nil)
	idBin := data.DataID.MarshalBinary()
	assert.EqualValues(t, idBin, digest)

	hasher.Reset()
	digest = hasher.Sum(nil)
	assert.NotEqualValues(t, digest, idBin)
}
