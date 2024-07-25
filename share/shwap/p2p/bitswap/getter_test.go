package bitswap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestEDSFromRows(t *testing.T) {
	edsIn := edstest.RandEDS(t, 8)
	roots, err := share.NewAxisRoots(edsIn)
	require.NoError(t, err)

	rows := make([]shwap.Row, edsIn.Width()/2)
	for i := range edsIn.Width() / 2 {
		rowShrs := edsIn.Row(i)[:edsIn.Width()/2]
		rows[i] = shwap.NewRow(rowShrs, shwap.Left)
	}

	edsOut, err := edsFromRows(roots, rows)
	require.NoError(t, err)
	require.True(t, edsIn.Equals(edsOut))
}
