package file

import (
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExtendAxisHalf(t *testing.T) {
	shares := sharetest.RandShares(t, 16)

	original := AxisHalf{
		Shares:   shares,
		IsParity: false,
	}

	extended, err := original.Extended()
	require.NoError(t, err)

	parity := AxisHalf{
		Shares:   extended[len(shares):],
		IsParity: true,
	}

	parityExtended, err := parity.Extended()
	require.NoError(t, err)

	require.Equal(t, extended, parityExtended)
}
