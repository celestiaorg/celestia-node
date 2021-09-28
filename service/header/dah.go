package header

import (
	"github.com/celestiaorg/rsmt2d"

	da "github.com/celestiaorg/celestia-core/pkg/da"
)

// DataAvailabilityHeaderFromExtendedData generates a DataAvailabilityHeader from the given data square.
func DataAvailabilityHeaderFromExtendedData(data *rsmt2d.ExtendedDataSquare) (da.DataAvailabilityHeader, error) {
	// generate the row and col roots using the EDS
	dah := da.DataAvailabilityHeader{
		RowsRoots:   data.RowRoots(),
		ColumnRoots: data.ColRoots(),
	}
	// generate the hash of the data using the new roots
	dah.Hash()

	return dah, nil
}
