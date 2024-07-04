package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestRowNamespaceDataID(t *testing.T) {
	edsSize := 4
	ns := sharetest.RandV0Namespace()

	id, err := NewRowNamespaceDataID(1, 1, ns, edsSize)
	require.NoError(t, err)

	data, err := id.MarshalBinary()
	require.NoError(t, err)

	sidOut, err := RowNamespaceDataIDFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, sidOut)

	err = sidOut.Verify(edsSize)
	require.NoError(t, err)
}
