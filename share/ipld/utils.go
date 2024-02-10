package ipld

import (
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share"
)

// FilterRootByNamespace returns the row roots from the given share.Root that contain the namespace.
func FilterRootByNamespace(root *share.Dah, namespace share.Namespace) []cid.Cid {
	rowRootCIDs := make([]cid.Cid, 0, len(root.RowRoots))
	for _, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			rowRootCIDs = append(rowRootCIDs, MustCidFromNamespacedSha256(row))
		}
	}
	return rowRootCIDs
}
