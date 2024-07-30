package shwap

import (
	"bytes"
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

func TestRowNamespaceDataIDReaderWriter(t *testing.T) {
	edsSize := 4
	ns := sharetest.RandV0Namespace()

	id, err := NewRowNamespaceDataID(1, 1, ns, edsSize)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	n, err := id.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RowNamespaceDataIDSize), n)

	rndidOut := RowNamespaceDataID{}
	n, err = rndidOut.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RowNamespaceDataIDSize), n)

	require.EqualValues(t, id, rndidOut)
}
