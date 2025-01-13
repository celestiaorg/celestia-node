package light

import (
	crand "crypto/rand"
	"maps"
	"math/big"
	"slices"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

// SamplingResult holds the available and remaining samples.
type SamplingResult struct {
	Available []shwap.SampleCoords `json:"available"`
	Remaining []shwap.SampleCoords `json:"remaining"`
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
func selectRandomSamples(squareSize, sampleCount int) []shwap.SampleCoords {
	total := squareSize * squareSize
	if sampleCount > total {
		sampleCount = total
	}

	samples := make(map[shwap.SampleCoords]struct{}, sampleCount)
	for len(samples) < sampleCount {
		s := shwap.SampleCoords{
			Row: randInt(squareSize),
			Col: randInt(squareSize),
		}
		samples[s] = struct{}{}
	}
	return slices.Collect(maps.Keys(samples))
}

func randInt(m int) int {
	n, err := crand.Int(crand.Reader, big.NewInt(int64(m)))
	if err != nil {
		panic(err) // won't panic as rand.Reader is endless
	}

	// n.Uint64() is safe as max is int
	return int(n.Uint64())
}
