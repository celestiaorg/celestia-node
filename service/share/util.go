package share

import "math"

// ProbabilityOfAvailability calculates the probability that the
// data square is available based on the given amount of
// samples collected.
//
// Formula: 1 - (0.75 ** amount of samples)
func ProbabilityOfAvailability(sampleAmount int) float64 {
	return 1 - math.Pow(0.75, float64(sampleAmount))
}
