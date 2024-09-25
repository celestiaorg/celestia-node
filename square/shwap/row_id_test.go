package shwap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRowID(t *testing.T) {
	edsSize := 4

	id, err := NewRowID(2, 1, edsSize)
	require.NoError(t, err)

	data, err := id.MarshalBinary()
	require.NoError(t, err)

	idOut, err := RowIDFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, idOut)

	err = idOut.Verify(edsSize)
	require.NoError(t, err)
	require.True(t, id.Equals(idOut))
}

func TestRowIDReaderWriter(t *testing.T) {
	edsSize := 4

	id, err := NewRowID(2, 1, edsSize)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	n, err := id.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RowIDSize), n)

	ridOut := RowID{}
	n, err = ridOut.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RowIDSize), n)

	require.EqualValues(t, id, ridOut)
}
