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
	square, _ := edstest.RandEDSWithNamespace(t, namespace, 8)

	nds, err := NewDataFromEDS(square, 1, namespace)
	require.NoError(t, err)
	nd := nds[0]

	data, err := nd.MarshalBinary()
	require.NoError(t, err)

	blk, err := nd.IPLDBlock()
	require.NoError(t, err)

	cid, err := nd.DataID.Cid()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), cid)

	ndOut := &Data{}
	err = ndOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, nd, ndOut)

	err = ndOut.Validate()
	require.NoError(t, err)
}
