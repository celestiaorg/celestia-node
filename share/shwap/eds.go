package shwap

import (
	"context"
	"fmt"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
)

// edsName is the name identifier for the Extended Data Square.
const edsName = "eds_v0"

// NOTE: There is no EDS container as it's already defined by rsmt2d and shwap.Accessor interface.
// TODO(@vgonkivs): add EDS container

// EDSData extracts shares for a specific namespace from an EDS, considering
// each row independently. It uses root to determine which rows to extract data from,
// avoiding the need to recalculate the row roots for each row.
func EDSData(
	ctx context.Context,
	eds Accessor,
	namespace libshare.Namespace,
) (NamespaceData, error) {
	roots, err := eds.AxisRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AxisRoots: %w", err)
	}
	rowIdxs, err := share.RowsWithNamespace(roots, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get row indexes: %w", err)
	}
	rows := make(NamespaceData, len(rowIdxs))
	for i, idx := range rowIdxs {
		rows[i], err = eds.RowNamespaceData(ctx, namespace, idx)
		if err != nil {
			return nil, fmt.Errorf("failed to process row %d: %w", idx, err)
		}
	}

	return rows, nil
}
