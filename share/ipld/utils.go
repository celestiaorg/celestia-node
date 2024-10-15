package ipld

import (
	"github.com/ipfs/go-cid"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
)

// FilterRootByNamespace returns the row roots from the given share.AxisRoots that contain the
// namespace.
func FilterRootByNamespace(root *share.AxisRoots, namespace libshare.Namespace) ([]cid.Cid, error) {
	rowRootCIDs := make([]cid.Cid, 0, len(root.RowRoots))
	for _, row := range root.RowRoots {
		outside, err := share.IsOutsideRange(namespace, row, row)
		if err != nil {
			return nil, err

		}

		if !outside {
			rowRootCIDs = append(rowRootCIDs, MustCidFromNamespacedSha256(row))
		}
	}
	return rowRootCIDs, nil
}
