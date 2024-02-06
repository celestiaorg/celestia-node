package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestDataID(t *testing.T) {
	ns := sharetest.RandV0Namespace()
	_, root := edstest.RandEDSWithNamespace(t, ns, 8, 4)

	id, err := NewDataID(1, 1, ns, root)
	require.NoError(t, err)

	cid := id.Cid()
	assert.EqualValues(t, dataCodec, cid.Prefix().Codec)
	assert.EqualValues(t, dataMultihashCode, cid.Prefix().MhType)
	assert.EqualValues(t, DataIDSize, cid.Prefix().MhLength)

	data, err := id.MarshalBinary()
	require.NoError(t, err)

	sidOut := DataID{}
	err = sidOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, sidOut)

	err = sidOut.Verify(root)
	require.NoError(t, err)
}
