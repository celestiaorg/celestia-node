package shwap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"
)

func TestNewRangeNamespaceDataID(t *testing.T) {
	edsSize := 16
	ns := libshare.RandomNamespace()

	rngid, err := NewRangeNamespaceDataID(
		EdsID{1},
		ns,
		SampleCoords{Row: 5, Col: 10},
		SampleCoords{7, 12},
		edsSize,
		true,
	)
	require.NoError(t, err)

	bin, err := rngid.MarshalBinary()
	require.NoError(t, err)

	rngidOut, err := RangeNamespaceDataIDFromBinary(bin)
	require.NoError(t, err)
	assert.EqualValues(t, rngid, rngidOut)

	err = rngid.Validate()
	require.NoError(t, err)
}

func TestRangeNamespaceDataIDReaderWriter(t *testing.T) {
	edsSize := 32
	ns := libshare.RandomNamespace()
	to, err := SampleCoordsFrom1DIndex(10, edsSize)
	require.NoError(t, err)
	rngid, err := NewRangeNamespaceDataID(EdsID{1}, ns, SampleCoords{Row: 0, Col: 1}, to, edsSize, false)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	n, err := rngid.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RangeNamespaceDataIDSize), n)

	rngidOut := RangeNamespaceDataID{}
	n, err = rngidOut.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RangeNamespaceDataIDSize), n)
	require.EqualValues(t, rngid, rngidOut)

	err = rngidOut.Validate()
	require.NoError(t, err)
}
