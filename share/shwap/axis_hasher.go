package shwap

import (
	"crypto/sha256"
	"fmt"
)

// AxisHasher implements hash.Hash interface for Samples.
type AxisHasher struct {
	sample Axis
}

// Write expects a marshaled ShareSample to validate.
func (sh *AxisHasher) Write(data []byte) (int, error) {
	if err := sh.sample.UnmarshalBinary(data); err != nil {
		err = fmt.Errorf("while unmarshaling Axis: %w", err)
		log.Error(err)
		return 0, err
	}

	if err := sh.sample.Validate(); err != nil {
		err = fmt.Errorf("while validating Axis: %w", err)
		log.Error(err)
		return 0, err
	}

	return len(data), nil
}

// Sum returns the "multihash" of the ShareSampleID.
func (sh *AxisHasher) Sum([]byte) []byte {
	sum, err := sh.sample.AxisID.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("while marshaling AxisID: %w", err)
		log.Error(err)
	}
	return sum
}

// Reset resets the Hash to its initial state.
func (sh *AxisHasher) Reset() {
	sh.sample = Axis{}
}

// Size returns the number of bytes Sum will return.
func (sh *AxisHasher) Size() int {
	return AxisIDSize
}

// BlockSize returns the hash's underlying block size.
func (sh *AxisHasher) BlockSize() int {
	return sha256.BlockSize
}
