package shwap

import (
	"bytes"
	"strconv"
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

func TestRangeNamespaceDataIDVerifyOutOfBounds(t *testing.T) {
	const odsSize = 4
	shares := odsSize * odsSize // total shares inside the ODS

	tests := []struct {
		name string
		from int
		to   int
	}{
		{"from at the share count", shares, shares + 1},
		{"from past the share count", shares + 3, shares + 4},
		{"to past the share count", shares - 1, shares + 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rngid := RangeNamespaceDataID{
				EdsID: EdsID{height: 1},
				From:  tc.from,
				To:    tc.to,
			}

			err := rngid.Verify(odsSize)
			require.Error(t, err)
			// out-of-bounds range indexes must be classifiable the same way as the
			// other shwap IDs, so callers can match on the shared sentinels.
			require.ErrorIs(t, err, ErrOutOfBounds)
			require.ErrorIs(t, err, ErrInvalidID)
			// the bound reported in the error is the ODS share count, not the ODS width.
			require.Contains(t, err.Error(), strconv.Itoa(shares))
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

	rngidV0in := RangeNamespaceDataIDV0{rngidOut}
	buf = bytes.NewBuffer(nil)
	n, err = rngidV0in.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RangeNamespaceDataIDV0Size), n)

	var rngidV0out RangeNamespaceDataIDV0
	n, err = rngidV0out.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RangeNamespaceDataIDV0Size), n)
	require.EqualValues(t, rngidV0in, rngidV0out)
}
