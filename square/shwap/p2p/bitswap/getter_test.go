package bitswap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/square"
	"github.com/celestiaorg/celestia-node/square/eds/edstest"
	"github.com/celestiaorg/celestia-node/square/shwap"
)

func TestEDSFromRows(t *testing.T) {
	edsIn := edstest.RandEDS(t, 8)
	roots, err := square.NewAxisRoots(edsIn)
	require.NoError(t, err)

	rows := make([]shwap.Row, edsIn.Width()/2)
	for i := range edsIn.Width() / 2 {
		rowShrs, err := share.FromBytes(edsIn.Row(i)[:edsIn.Width()/2])
		require.NoError(t, err)
		rows[i] = shwap.NewRow(rowShrs, shwap.Left)
	}

	edsOut, err := edsFromRows(roots, rows)
	require.NoError(t, err)
	require.True(t, edsIn.Equals(edsOut))
}
