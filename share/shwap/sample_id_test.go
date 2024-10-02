package shwap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSampleID(t *testing.T) {
	edsSize := 4

	id, err := NewSampleID(1, 1, 1, edsSize)
	require.NoError(t, err)

	data, err := id.MarshalBinary()
	require.NoError(t, err)

	idOut, err := SampleIDFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, idOut)

	err = idOut.Verify(edsSize)
	require.NoError(t, err)
	require.True(t, id.Equals(idOut))
}

func TestSampleIDReaderWriter(t *testing.T) {
	edsSize := 4

	id, err := NewSampleID(1, 1, 1, edsSize)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	n, err := id.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(SampleIDSize), n)

	sidOut := SampleID{}
	n, err = sidOut.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(SampleIDSize), n)

	require.EqualValues(t, id, sidOut)
}
