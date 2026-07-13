package shwap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSampleID(t *testing.T) {
	edsSize := 4

	id, err := NewSampleID(1, SampleCoords{Col: 1}, edsSize)
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

	id, err := NewSampleID(1, SampleCoords{Col: 1}, edsSize)
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

func TestSampleCoords(t *testing.T) {
	edsSize := 16

	rawIdx := 13 * 16
	idxIn, err := SampleCoordsFrom1DIndex(rawIdx, edsSize)
	require.NoError(t, err)

	idxOut, err := SampleCoordsAs1DIndex(idxIn, edsSize)
	require.NoError(t, err)
	assert.Equal(t, rawIdx, idxOut)
}

func TestSampleCoordsFrom1DIndexBounds(t *testing.T) {
	const squareSize = 16
	last := squareSize*squareSize - 1

	// the last valid index maps to the last cell.
	coords, err := SampleCoordsFrom1DIndex(last, squareSize)
	require.NoError(t, err)
	assert.Equal(t, SampleCoords{Row: squareSize - 1, Col: squareSize - 1}, coords)

	// one past the end must be rejected, not silently mapped to the
	// non-existent row squareSize.
	_, err = SampleCoordsFrom1DIndex(squareSize*squareSize, squareSize)
	require.ErrorIs(t, err, ErrOutOfBounds)

	// a negative index must be rejected, not silently mapped to a negative
	// row/col.
	_, err = SampleCoordsFrom1DIndex(-1, squareSize)
	require.ErrorIs(t, err, ErrOutOfBounds)
}
