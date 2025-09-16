package shwap

import (
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"
)

func TestExtendAxisHalf(t *testing.T) {
	shares, err := libshare.RandShares(16)
	require.NoError(t, err)

	original := AxisHalf{
		Shares:   shares,
		IsParity: false,
	}

	extended, err := original.Extended()
	require.NoError(t, err)
	require.Len(t, extended, len(shares)*2)

	parity := AxisHalf{
		Shares:   extended[len(shares):],
		IsParity: true,
	}

	parityExtended, err := parity.Extended()
	require.NoError(t, err)

	require.Equal(t, extended, parityExtended)
}
