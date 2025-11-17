package shwap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRangeNamespaceDataID(t *testing.T) {
	tests := []struct {
		name  string
		from  int
		to    int
		total int
		valid bool
	}{
		{"valid indexes", 4, 11, 16, true},
		{"valid indexes: big numbers", 30000000, 35000000, 50000000, true},
		{"start index out of ods", 17, 22, 4, false},
		{"end index out of ods", 1, 22, 4, false},
		{"invalid: negative index", -1, 6, 16, false},
		{"invalid: to before from", 6, 3, 16, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rngid, err := NewRangeNamespaceDataID(
				EdsID{1},
				tc.from,
				tc.to,
				tc.total/2,
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
	rngid, err := NewRangeNamespaceDataID(EdsID{1}, 1, 10, edsSize/2)
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
