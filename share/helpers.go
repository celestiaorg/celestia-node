package share

import (
	"bytes"

	"github.com/celestiaorg/rsmt2d"
)

// TODO(Wondertan): All these helpers should be methods on rsmt2d.EDS

// ExtractODS returns the original shares of the given ExtendedDataSquare. This
// is a helper function for circumstances where AddShares must be used after the EDS has already
// been generated.
func ExtractODS(eds *rsmt2d.ExtendedDataSquare) []Share {
	origWidth := eds.Width() / 2
	origShares := make([][]byte, origWidth*origWidth)
	for i := uint(0); i < origWidth; i++ {
		row := eds.Row(i)
		for j := uint(0); j < origWidth; j++ {
			origShares[(i*origWidth)+j] = row[j]
		}
	}
	return origShares
}

// ExtractEDS takes an EDS and extracts all shares from it in a flattened slice(row by row).
func ExtractEDS(eds *rsmt2d.ExtendedDataSquare) []Share {
	flattenedEDSSize := eds.Width() * eds.Width()
	out := make([][]byte, flattenedEDSSize)
	count := 0
	for i := uint(0); i < eds.Width(); i++ {
		for _, share := range eds.Row(i) {
			out[count] = share
			count++
		}
	}
	return out
}

// EqualEDS check whether two given EDSes are equal.
// TODO(Wondertan): Propose use of int by default instead of uint for the sake convenience and
// Golang practices
func EqualEDS(a *rsmt2d.ExtendedDataSquare, b *rsmt2d.ExtendedDataSquare) bool {
	if a.Width() != b.Width() {
		return false
	}

	for i := uint(0); i < a.Width(); i++ {
		ar, br := a.Row(i), b.Row(i)
		for j := 0; j < len(ar); j++ {
			if !bytes.Equal(ar[j], br[j]) {
				return false
			}
		}
	}

	return true
}
