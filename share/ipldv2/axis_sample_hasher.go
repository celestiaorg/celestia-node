package ipldv2

import (
	"crypto/sha256"
	"fmt"
)

// AxisSampleHasher implements hash.Hash interface for Samples.
type AxisSampleHasher struct {
	sample AxisSample
}

// Write expects a marshaled ShareSample to validate.
func (sh *AxisSampleHasher) Write(data []byte) (int, error) {
	if err := sh.sample.UnmarshalBinary(data); err != nil {
		err = fmt.Errorf("while unmarshaling ShareSample: %w", err)
		log.Error(err)
		return 0, err
	}

	if err := sh.sample.Validate(); err != nil {
		err = fmt.Errorf("while validating ShareSample: %w", err)
		log.Error(err)
		return 0, err
	}

	return len(data), nil
}

// Sum returns the "multihash" of the ShareSampleID.
func (sh *AxisSampleHasher) Sum([]byte) []byte {
	sum, err := sh.sample.ID.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("while marshaling ShareSampleID")
		log.Error(err)
	}
	return sum
}

// Reset resets the Hash to its initial state.
func (sh *AxisSampleHasher) Reset() {
	sh.sample = AxisSample{}
}

// Size returns the number of bytes Sum will return.
func (sh *AxisSampleHasher) Size() int {
	return AxisSampleIDSize
}

// BlockSize returns the hash's underlying block size.
func (sh *AxisSampleHasher) BlockSize() int {
	return sha256.BlockSize
}
