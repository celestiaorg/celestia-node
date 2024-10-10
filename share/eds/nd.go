package eds

import (
	"context"
	"fmt"

	gosquare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// NamespaceData extracts shares for a specific namespace from an EDS, considering
// each row independently. It uses root to determine which rows to extract data from,
// avoiding the need to recalculate the row roots for each row.
func NamespaceData(
	ctx context.Context,
	eds Accessor,
	namespace gosquare.Namespace,
) (shwap.NamespaceData, error) {
	roots, err := eds.AxisRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AxisRoots: %w", err)
	}
	rowIdxs := share.RowsWithNamespace(roots, namespace)
	rows := make(shwap.NamespaceData, len(rowIdxs))
	for i, idx := range rowIdxs {
		rows[i], err = eds.RowNamespaceData(ctx, namespace, idx)
		if err != nil {
			return nil, fmt.Errorf("failed to process row %d: %w", idx, err)
		}
	}

	return rows, nil
}
