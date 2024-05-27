package share

import (
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"
)

// Root represents root commitment to multiple Shares.
// In practice, it is a commitment to all the Data in a square.
type Root = da.DataAvailabilityHeader

// NewRoot generates Root(DataAvailabilityHeader) using the
// provided extended data square.
func NewRoot(eds *rsmt2d.ExtendedDataSquare) (*Root, error) {
	dah, err := da.NewDataAvailabilityHeader(eds)
	if err != nil {
		return nil, err
	}
	return &dah, nil
}

// RowsWithNamespace inspects the Root for the Namespace and provides
// a slices of Row indexes containing the namespace.
func RowsWithNamespace(root *Root, namespace Namespace) (idxs []int) {
	for i, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			idxs = append(idxs, i)
		}
	}
	return
}
