package eds

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// NamespacedData extracts shares for a specific namespace from an EDS, considering
// each row independently. It uses root to determine which rows to extract data from,
// avoiding the need to recalculate the row roots for each row.
func NamespacedData(
	ctx context.Context,
	eds Accessor,
	namespace share.Namespace,
) (shwap.NamespacedData, error) {
	roots, err := eds.AxisRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AxisRoots: %w", err)
	}
	rowIdxs := share.RowsWithNamespace(roots, namespace)
	rows := make(shwap.NamespacedData, len(rowIdxs))
	for i, idx := range rowIdxs {
		rows[i], err = eds.RowNamespaceData(ctx, namespace, idx)
		if err != nil {
			return nil, fmt.Errorf("failed to process row %d: %w", idx, err)
		}
	}

	return rows, nil
}
