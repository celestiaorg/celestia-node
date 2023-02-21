package utils

import "math"

// SquareSize returns the size of the square based on the given amount of shares.
func SquareSize(lenShares int) uint64 {
	return uint64(math.Sqrt(float64(lenShares)))
}
