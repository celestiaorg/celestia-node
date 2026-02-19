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

func TestSampleCoordsFrom1DIndex_Bounds(t *testing.T) {
	squareSize := 8

	t.Run("negative index", func(t *testing.T) {
		_, err := SampleCoordsFrom1DIndex(-1, squareSize)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidID)
	})

	t.Run("last valid index", func(t *testing.T) {
		lastIdx := squareSize*squareSize - 1 // 63
		coords, err := SampleCoordsFrom1DIndex(lastIdx, squareSize)
		require.NoError(t, err)
		assert.Equal(t, squareSize-1, coords.Row)
		assert.Equal(t, squareSize-1, coords.Col)
	})

	t.Run("index equals square size", func(t *testing.T) {
		_, err := SampleCoordsFrom1DIndex(squareSize*squareSize, squareSize)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrOutOfBounds)
	})

	t.Run("index exceeds square size", func(t *testing.T) {
		_, err := SampleCoordsFrom1DIndex(squareSize*squareSize+1, squareSize)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrOutOfBounds)
	})
}
