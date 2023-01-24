package getters

import (
	"context"
	"fmt"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

var (
	tracer = otel.Tracer("share/getters")
	log    = logging.Logger("share/getters")
)

// filterRootsByNamespace returns the row roots from the given share.Root that contain the passed
// namespace ID.
func filterRootsByNamespace(root *share.Root, nID namespace.ID) []cid.Cid {
	rowRootCIDs := make([]cid.Cid, 0, len(root.RowsRoots))
	for _, row := range root.RowsRoots {
		if !nID.Less(nmt.MinNamespace(row, nID.Size())) && nID.LessOrEqual(nmt.MaxNamespace(row, nID.Size())) {
			rowRootCIDs = append(rowRootCIDs, ipld.MustCidFromNamespacedSha256(row))
		}
	}
	return rowRootCIDs
}

// collectSharesByNamespace collects NamespaceShares within the given namespace ID from the given
// share.Root.
func collectSharesByNamespace(
	ctx context.Context,
	bg blockservice.BlockGetter,
	root *share.Root,
	nID namespace.ID,
) (shares share.NamespacedShares, err error) {
	ctx, span := tracer.Start(ctx, "collect-shares-by-namespace", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.String("nid", nID.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	rootCIDs := filterRootsByNamespace(root, nID)
	if len(rootCIDs) == 0 {
		return nil, nil
	}

	errGroup, ctx := errgroup.WithContext(ctx)
	shares = make([]share.NamespacedRow, len(rootCIDs))
	for i, rootCID := range rootCIDs {
		// shadow loop variables, to ensure correct values are captured
		i, rootCID := i, rootCID
		errGroup.Go(func() error {
			proof := new(ipld.Proof)
			row, err := share.GetSharesByNamespace(ctx, bg, rootCID, nID, len(root.RowsRoots), proof)
			shares[i] = share.NamespacedRow{
				Shares: row,
				Proof:  proof,
			}
			if err != nil {
				return fmt.Errorf("retrieving nID %x for row %x: %w", nID, rootCID, err)
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, err
	}

	return shares, nil
}

func verifyNIDSize(nID namespace.ID) error {
	if len(nID) != share.NamespaceSize {
		return fmt.Errorf("expected namespace ID of size %d, got %d",
			share.NamespaceSize, len(nID))
	}
	return nil
}
