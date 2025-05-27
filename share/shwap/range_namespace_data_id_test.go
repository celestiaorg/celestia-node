package shwap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"
)

func TestNewRangeNamespaceDataID(t *testing.T) {
	tests := []struct {
		name  string
		from  SampleCoords
		to    SampleCoords
		total int
		valid bool
	}{
		{"valid coordinates", SampleCoords{5, 10}, SampleCoords{7, 12}, 16, true},
		{"invalid: negative row", SampleCoords{-1, 5}, SampleCoords{7, 12}, 16, false},
		{"invalid: to before from", SampleCoords{7, 12}, SampleCoords{5, 10}, 16, false},
		{"invalid: out of bounds row", SampleCoords{0, 0}, SampleCoords{17, 0}, 16, false},
		{"invalid: out of bounds col", SampleCoords{0, 0}, SampleCoords{15, 17}, 16, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ns := libshare.RandomNamespace()
			rngid, err := NewRangeNamespaceDataID(
				EdsID{1},
				ns,
				tc.from,
				tc.to,
				tc.total,
				true,
			)
			if tc.valid {
				require.NoError(t, err)
				bin, err := rngid.MarshalBinary()
				require.NoError(t, err)
				rngidOut, err := RangeNamespaceDataIDFromBinary(bin)
				require.NoError(t, err)
				require.EqualValues(t, rngid, rngidOut)
				require.NoError(t, rngid.Validate())
			} else {
				require.Error(t, err)
			}
		})
	}
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
