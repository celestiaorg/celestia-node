package eds

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
)

// rawODS returns odsSize*odsSize random shares and their row-major byte serialization.
func rawODS(t *testing.T, odsSize int) ([]byte, []libshare.Share) {
	t.Helper()
	shares, err := libshare.RandShares(odsSize * odsSize)
	require.NoError(t, err)

	buf := make([]byte, 0, len(shares)*libshare.ShareSize)
	for _, sh := range shares {
		buf = append(buf, sh.ToBytes()...)
	}
	return buf, shares
}

func TestReadShares_Full(t *testing.T) {
	const odsSize = 4
	raw, want := rawODS(t, odsSize)

	got, err := ReadShares(bytes.NewReader(raw), libshare.ShareSize, odsSize)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestReadShares_EOFMidRowBoundary(t *testing.T) {
	const odsSize = 4
	raw, want := rawODS(t, odsSize)

	// end mid-row on a share boundary; the rest must become tail padding
	const kept = 2*odsSize + 2
	truncated := raw[:kept*libshare.ShareSize]

	got, err := ReadShares(bytes.NewReader(truncated), libshare.ShareSize, odsSize)
	require.NoError(t, err)

	for i := range got {
		if i < kept {
			require.Equal(t, want[i], got[i], "share %d", i)
		} else {
			require.Equal(t, libshare.TailPaddingShare(), got[i], "share %d should be tail padding", i)
		}
	}
}

func TestReadShares_EOFRowBoundary(t *testing.T) {
	const odsSize = 4
	raw, want := rawODS(t, odsSize)

	// end exactly on a row boundary; remaining rows become tail padding
	const kept = 2 * odsSize
	truncated := raw[:kept*libshare.ShareSize]

	got, err := ReadShares(bytes.NewReader(truncated), libshare.ShareSize, odsSize)
	require.NoError(t, err)

	for i := range got {
		if i < kept {
			require.Equal(t, want[i], got[i], "share %d", i)
		} else {
			require.Equal(t, libshare.TailPaddingShare(), got[i], "share %d", i)
		}
	}
}

func TestReadShares_MidShareTruncationErrors(t *testing.T) {
	const odsSize = 4
	raw, _ := rawODS(t, odsSize)

	// end mid-share; must error rather than pad
	truncated := raw[:libshare.ShareSize+10]

	_, err := ReadShares(bytes.NewReader(truncated), libshare.ShareSize, odsSize)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}
