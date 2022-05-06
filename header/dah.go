package header

import (
	"github.com/celestiaorg/rsmt2d"
)

// DataAvailabilityHeaderFromExtendedData generates a DataAvailabilityHeader from the given data square.
// TODO @renaynay: use da.NewDataAvailabilityHeader
func DataAvailabilityHeaderFromExtendedData(data *rsmt2d.ExtendedDataSquare) (DataAvailabilityHeader, error) {
	// generate the row and col roots using the EDS
	dah := DataAvailabilityHeader{
		RowsRoots:   data.RowRoots(),
		ColumnRoots: data.ColRoots(),
	}
	// generate the hash of the data using the new roots
	dah.Hash()

	return dah, nil
}
