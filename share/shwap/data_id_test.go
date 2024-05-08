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

	data := id.MarshalBinary()
	sidOut, err := DataIDFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, sidOut)

	err = sidOut.Verify(root)
	require.NoError(t, err)
}
