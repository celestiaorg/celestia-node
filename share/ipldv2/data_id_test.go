package ipldv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

// TODO: Add test that AxisType is not serialized
func TestDataID(t *testing.T) {
	ns := sharetest.RandV0Namespace()
	_, root := edstest.RandEDSWithNamespace(t, ns, 4)

	sid := NewDataID(2, root, 1, ns)
	id, err := sid.Cid()
	require.NoError(t, err)

	assert.EqualValues(t, dataCodec, id.Prefix().Codec)
	assert.EqualValues(t, dataMultihashCode, id.Prefix().MhType)
	assert.EqualValues(t, DataIDSize, id.Prefix().MhLength)

	data, err := sid.MarshalBinary()
	require.NoError(t, err)

	sidOut := DataID{}
	err = sidOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, sid, sidOut)

	err = sidOut.Validate()
	require.NoError(t, err)
}
