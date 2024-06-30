package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestRowID(t *testing.T) {
	square := edstest.RandEDS(t, 2)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	id, err := NewRowID(2, 1, root)
	require.NoError(t, err)

	cid := id.Cid()
	assert.EqualValues(t, rowCodec, cid.Prefix().Codec)
	assert.EqualValues(t, rowMultihashCode, cid.Prefix().MhType)
	assert.EqualValues(t, RowIDSize, cid.Prefix().MhLength)

	data, err := id.MarshalBinary()
	require.NoError(t, err)

	idOut := RowID{}
	err = idOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, idOut)

	err = idOut.Verify(root)
	require.NoError(t, err)
}
