package shwap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"
)

func TestNamespaceDataID(t *testing.T) {
	ns := libshare.RandomNamespace()

	id, err := NewNamespaceDataID(1, ns)
	require.NoError(t, err)

	data, err := id.MarshalBinary()
	require.NoError(t, err)

	sidOut, err := NamespaceDataIDFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, sidOut)

	err = sidOut.Validate()
	require.NoError(t, err)
	require.True(t, id.Equals(sidOut))
}

func TestNamespaceDataIDReaderWriter(t *testing.T) {
	ns := libshare.RandomNamespace()

	id, err := NewNamespaceDataID(1, ns)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	n, err := id.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(NamespaceDataIDSize), n)

	ndidOut := NamespaceDataID{}
	n, err = ndidOut.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(NamespaceDataIDSize), n)

	require.EqualValues(t, id, ndidOut)
}
