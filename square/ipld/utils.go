package ipld

import (
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/square"
)

// FilterRootByNamespace returns the row roots from the given square.AxisRoots that contain the
// namespace.
func FilterRootByNamespace(root *square.AxisRoots, namespace share.Namespace) []cid.Cid {
	rowRootCIDs := make([]cid.Cid, 0, len(root.RowRoots))
	for _, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			rowRootCIDs = append(rowRootCIDs, MustCidFromNamespacedSha256(row))
		}
	}
	return rowRootCIDs
}
