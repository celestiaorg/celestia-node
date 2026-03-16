package share

import (
	"errors"

	blsfr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

var (
	// ErrCDASingularMatrix indicates that the coding matrix is not invertible
	// (insufficient independent fragments).
	ErrCDASingularMatrix = errors.New("cda: singular coding matrix")
)

// SolveLinearSystem solves G * x = y over the BLS12-381 scalar field.
//
// - G must be a square n×n matrix (n > 0).
// - y must have length n.
//
// It returns x of length n, or ErrCDASingularMatrix if G is not invertible.
//
// Note: This is the core “engine” for CDA decoding. Higher-level code is
// responsible for mapping shares/pieces/fragments into field elements.
func SolveLinearSystem(G [][]blsfr.Element, y []blsfr.Element) ([]blsfr.Element, error) {
	n := len(G)
	if n == 0 || len(y) != n {
		return nil, ErrCDASingularMatrix
	}
	for i := 0; i < n; i++ {
		if len(G[i]) != n {
			return nil, ErrCDASingularMatrix
		}
	}

	// Build augmented matrix [G | y] as copies so caller retains ownership.
	aug := make([][]blsfr.Element, n)
	for i := 0; i < n; i++ {
		row := make([]blsfr.Element, n+1)
		copy(row[:n], G[i])
		row[n] = y[i]
		aug[i] = row
	}

	// Gaussian elimination to reduced row echelon form.
	for col := 0; col < n; col++ {
		// Find a pivot row at or below col with non-zero at aug[r][col].
		pivot := -1
		for r := col; r < n; r++ {
			if !aug[r][col].IsZero() {
				pivot = r
				break
			}
		}
		if pivot == -1 {
			return nil, ErrCDASingularMatrix
		}

		// Swap pivot row into place if needed.
		if pivot != col {
			aug[pivot], aug[col] = aug[col], aug[pivot]
		}

		// Normalize pivot row so pivot element becomes 1.
		var inv blsfr.Element
		inv.Inverse(&aug[col][col])
		for c := col; c < n+1; c++ {
			aug[col][c].Mul(&aug[col][c], &inv)
		}

		// Eliminate this column from all other rows.
		for r := 0; r < n; r++ {
			if r == col {
				continue
			}
			if aug[r][col].IsZero() {
				continue
			}
			factor := aug[r][col] // copy
			// row_r = row_r - factor * row_col
			for c := col; c < n+1; c++ {
				var tmp blsfr.Element
				tmp.Mul(&factor, &aug[col][c])
				aug[r][c].Sub(&aug[r][c], &tmp)
			}
			aug[r][col].SetZero()
		}
	}

	// Extract solution.
	x := make([]blsfr.Element, n)
	for i := 0; i < n; i++ {
		x[i] = aug[i][n]
	}
	return x, nil
}

