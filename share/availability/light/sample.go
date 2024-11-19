package light

import (
	crand "crypto/rand"
	"math/big"

	"golang.org/x/exp/maps"
)

// SamplingResult holds the available and remaining samples.
type SamplingResult struct {
	Available []Sample `json:"available"`
	Remaining []Sample `json:"remaining"`
}

// Sample represents a coordinate in a 2D data square.
type Sample struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// NewSamplingResult creates a new SamplingResult with randomly selected samples.
func NewSamplingResult(squareSize, sampleCount int) *SamplingResult {
	total := squareSize * squareSize
	if sampleCount > total {
		sampleCount = total
	}

	samples := selectRandomSamples(squareSize, sampleCount)
	return &SamplingResult{
		Remaining: samples,
	}
}

// selectRandomSamples randomly picks unique coordinates from a square of given size.
func selectRandomSamples(squareSize, sampleCount int) []Sample {
	total := squareSize * squareSize
	if sampleCount > total {
		sampleCount = total
	}

	samples := make(map[Sample]struct{}, sampleCount)
	for len(samples) < sampleCount {
		s := Sample{
			Row: randInt(squareSize),
			Col: randInt(squareSize),
		}
		samples[s] = struct{}{}
	}
	return maps.Keys(samples)
}

func randInt(m int) int {
	n, err := crand.Int(crand.Reader, big.NewInt(int64(m)))
	if err != nil {
		panic(err) // won't panic as rand.Reader is endless
	}

	// n.Uint64() is safe as max is int
	return int(n.Uint64())
}
