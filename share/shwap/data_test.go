package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestData(t *testing.T) {
	namespace := sharetest.RandV0Namespace()
	square, root := edstest.RandEDSWithNamespace(t, namespace, 16, 8)

	nds, err := NewDataFromEDS(square, 1, namespace)
	require.NoError(t, err)
	nd := nds[0]

	data, err := nd.MarshalBinary()
	require.NoError(t, err)

	blk, err := nd.IPLDBlock()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), nd.Cid())

	dataOut := &Data{}
	err = dataOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, nd, dataOut)

	err = dataOut.Verify(root)
	require.NoError(t, err)
}
