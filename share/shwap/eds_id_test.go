package shwap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEdsID(t *testing.T) {
	id, err := NewEdsID(2)
	require.NoError(t, err)

	data, err := id.MarshalBinary()
	require.NoError(t, err)

	idOut, err := EdsIDFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, idOut)

	err = idOut.Validate()
	require.NoError(t, err)
	require.True(t, id.Equals(idOut))
}

func TestEdsIDReaderWriter(t *testing.T) {
	id, err := NewEdsID(2)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	n, err := id.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(EdsIDSize), n)

	eidOut := EdsID{}
	n, err = eidOut.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(EdsIDSize), n)

	require.EqualValues(t, id, eidOut)
}
