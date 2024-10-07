package ipld

import (
	"github.com/ipfs/go-cid"

	gosquare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
)

// FilterRootByNamespace returns the row roots from the given share.AxisRoots that contain the
// namespace.
func FilterRootByNamespace(root *share.AxisRoots, namespace gosquare.Namespace) []cid.Cid {
	rowRootCIDs := make([]cid.Cid, 0, len(root.RowRoots))
	for _, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			rowRootCIDs = append(rowRootCIDs, MustCidFromNamespacedSha256(row))
		}
	}
	return rowRootCIDs
}
