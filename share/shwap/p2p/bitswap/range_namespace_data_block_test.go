package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestRangeNamespaceData_FetchRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := libshare.RandomNamespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 8)
	exchange := newExchangeOverEDS(ctx, t, eds)
	blk, err := NewEmptyRangeNamespaceDataBlock(1, namespace,
		shwap.SampleCoords{Row: 0, Col: 0},
		shwap.SampleCoords{Row: 2, Col: 2},
		16,
		false,
	)
	require.NoError(t, err)

	err = Fetch(ctx, exchange, root, []Block{blk})
	require.NoError(t, err)

	err = blk.Container.Verify(
		namespace,
		shwap.SampleCoords{Row: 0, Col: 0},
		shwap.SampleCoords{Row: 2, Col: 2},
		root.Hash(),
	)
	require.NoError(t, err)
}
