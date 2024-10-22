// TODO(@Wondertan): Instead of doing sampling over the coordinates do a random walk over NMT trees.
package light

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"math/big"
)

// Sample is a point in 2D space over square.
type Sample struct {
	Row, Col uint16
}

// SampleSquare randomly picks *num* unique points from the given *width* square
// and returns them as samples.
func SampleSquare(squareWidth, num int) ([]Sample, error) {
	ss := newSquareSampler(squareWidth, num)
	err := ss.generateSample(num)
	if err != nil {
		return nil, err
	}
	return ss.samples(), nil
}

type squareSampler struct {
	squareWidth int
	smpls       map[Sample]struct{}
}

func newSquareSampler(squareWidth, expectedSamples int) *squareSampler {
	return &squareSampler{
		squareWidth: squareWidth,
		smpls:       make(map[Sample]struct{}, expectedSamples),
	}
}

// generateSample randomly picks unique point on a 2D spaces.
func (ss *squareSampler) generateSample(num int) error {
	if num > ss.squareWidth*ss.squareWidth {
		num = ss.squareWidth
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

func randInt(max int) uint16 {
	n, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err) // won't panic as rand.Reader is endless
	}

	return uint16(n.Uint64())
}

// encodeSamples encodes a slice of samples into a byte slice using little endian encoding.
func encodeSamples(samples []Sample) []byte {
	bs := make([]byte, 0, len(samples)*4)
	for _, s := range samples {
		bs = binary.LittleEndian.AppendUint16(bs, s.Row)
		bs = binary.LittleEndian.AppendUint16(bs, s.Col)
	}
	return bs
}

// decodeSamples decodes a byte slice into a slice of samples.
func decodeSamples(bs []byte) ([]Sample, error) {
	if len(bs)%4 != 0 {
		return nil, errors.New("invalid byte slice length")
	}

	samples := make([]Sample, 0, len(bs)/4)
	for i := 0; i < len(bs); i += 4 {
		samples = append(samples, Sample{
			Row: binary.LittleEndian.Uint16(bs[i : i+2]),
			Col: binary.LittleEndian.Uint16(bs[i+2 : i+4]),
		})
	}
	return samples, nil
}
