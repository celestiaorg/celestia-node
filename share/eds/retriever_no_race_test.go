//go:build !race

package eds

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

func TestRetriever_ByzantineError(t *testing.T) {
	const width = 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bserv := ipld.NewMemBlockservice()
	shares := edstest.RandEDS(t, width).Flattened()
	_, err := ipld.ImportShares(ctx, shares, bserv)
	require.NoError(t, err)

	// corrupt shares so that eds erasure coding does not match
	copy(shares[14][share.NamespaceSize:], shares[15][share.NamespaceSize:])

	// import corrupted eds
	batchAdder := ipld.NewNmtNodeAdder(ctx, bserv, ipld.MaxSizeBatchOption(width*2))
	attackerEDS, err := rsmt2d.ImportExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(width),
			nmt.NodeVisitor(batchAdder.Visit)),
	)
	require.NoError(t, err)
	err = batchAdder.Commit()
	require.NoError(t, err)

	// ensure we rcv an error
	roots, err := share.NewAxisRoots(attackerEDS)
	require.NoError(t, err)
	r := NewRetriever(bserv)
	_, err = r.Retrieve(ctx, roots)
	var errByz *byzantine.ErrByzantine
	require.ErrorAs(t, err, &errByz)
}
