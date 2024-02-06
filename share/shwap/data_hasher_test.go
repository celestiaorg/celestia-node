package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

// TODO(@walldiss):
// FIX: hasher test succeed, while logging unmarshal error: "unmarshaling Data: proto: Data: wiretype end group for non-group"
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

	dataVerifiers.Add(data.DataID, func(data Data) error {
		return data.Verify(root)
	})

	dat, err := data.MarshalBinary()
	require.NoError(t, err)

	n, err := hasher.Write(dat)
	require.NoError(t, err)
	assert.EqualValues(t, len(dat), n)

	digest := hasher.Sum(nil)
	id, err := data.DataID.MarshalBinary()
	require.NoError(t, err)
	assert.EqualValues(t, id, digest)

	hasher.Reset()
	digest = hasher.Sum(nil)
	assert.NotEqualValues(t, digest, id)
}
