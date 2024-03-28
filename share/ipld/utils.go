package ipld

import (
	"github.com/celestiaorg/celestia-node/share"
)

// FilterRootByNamespace returns the row roots from the given share.Root that contain the namespace.
func FilterRootByNamespace(root *share.Root, namespace share.Namespace) [][]byte {
	rowRootCIDs := make([][]byte, 0, len(root.RowRoots))
	for _, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			rowRootCIDs = append(rowRootCIDs, row)
		}
	}
	return rowRootCIDs
}
