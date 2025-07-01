package eds

import (
	"context"
	"fmt"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"time"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// NamespaceData extracts shares for a specific namespace from an EDS, considering
// each row independently. It uses root to determine which rows to extract data from,
// avoiding the need to recalculate the row roots for each row.
func NamespaceData(
	ctx context.Context,
	eds Accessor,
	namespace libshare.Namespace,
) (shwap.NamespaceData, error) {
	startTime := time.Now()
	roots, err := eds.AxisRoots(ctx)
	fmt.Println("INSIDE NamespaceData: eds.AxisRoots took  ", time.Since(startTime).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("failed to get AxisRoots: %w", err)
	}

	startTime = time.Now()
	rowIdxs, err := share.RowsWithNamespace(roots, namespace)
	fmt.Println("INSIDE NamespaceData: share.RowsWithNamespace took  ", time.Since(startTime).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("failed to get row indexes: %w", err)
	}
	rows := make(shwap.NamespaceData, len(rowIdxs))
	for i, idx := range rowIdxs {
		startTime = time.Now()
		rows[i], err = eds.RowNamespaceData(ctx, namespace, idx)
		fmt.Println("INSIDE NamespaceData: eds.RowNamespaceData took  ", time.Since(startTime).Milliseconds())
		if err != nil {
			return nil, fmt.Errorf("failed to process row %d: %w", idx, err)
		}
	}

	return rows, nil
}
