package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/testing/edstest"
)

func TestEdsID(t *testing.T) {
	square := edstest.RandEDS(t, 2)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	id, err := NewEdsID(2, root)
	require.NoError(t, err)

	data := id.MarshalBinary()
	idOut, err := EdsIDFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, idOut)

	err = idOut.Verify(root)
	require.NoError(t, err)
}
