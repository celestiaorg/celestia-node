package das

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
)

// Sample is a point in 2D space over square.
type Sample struct {
	Row, Col int
}

// SampleSquare randomly picks *num* unique points from arbitrary *width* square
// and returns them as samples.
func SampleSquare(squareWidth int, num int) ([]Sample, error) {
	ss := newSquareSampler(squareWidth, num)
	err := ss.sample(num)
	if err != nil {
		return nil, err
	}
	return ss.samples(), nil
}

type squareSampler struct {
	squareWidth int
	smpls       map[Sample]struct{}
}

func newSquareSampler(squareWidth int, expectedSamples int) *squareSampler {
	return &squareSampler{
		squareWidth: squareWidth,
		smpls:       make(map[Sample]struct{}, expectedSamples),
	}
}

func (ss *squareSampler) sample(num int) error {
	if num > ss.squareWidth*ss.squareWidth {
		return fmt.Errorf("number of samples must be less than square width")
	}

	done := 0
	for done < num {
		s := Sample{
			Row: randInt(ss.squareWidth),
			Col: randInt(ss.squareWidth),
		}

		if _, ok := ss.smpls[s]; ok {
			continue
		}

		done++
		ss.smpls[s] = struct{}{}
	}

	return nil
}

func (ss *squareSampler) samples() []Sample {
	samples := make([]Sample, 0, len(ss.smpls))
	for s := range ss.smpls {
		samples = append(samples, s)
	}
	return samples
}

func randInt(max int) int {
	n, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err) // won't panic as rand.Reader is endless
	}

	return int(n.Int64())
}