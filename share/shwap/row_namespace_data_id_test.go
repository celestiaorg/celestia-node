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

	id, err := NewRowNamespaceDataID(1, 1, ns, root)
	require.NoError(t, err)

	data, err := id.MarshalBinary()
	require.NoError(t, err)

	sidOut, err := RowNamespaceDataIDFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, sidOut)

	err = sidOut.Validate(root)
	require.NoError(t, err)
}
