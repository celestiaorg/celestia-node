package ipldv2

import (
	"crypto/sha256"
	"fmt"
)

// SampleHasher implements hash.Hash interface for Samples.
type SampleHasher struct {
	sample Sample
}

// Write expects a marshaled Sample to validate.
func (sh *SampleHasher) Write(data []byte) (int, error) {
	if err := sh.sample.UnmarshalBinary(data); err != nil {
		err = fmt.Errorf("while unmarshaling Sample: %w", err)
		log.Error(err)
		return 0, err
	}

	if err := sh.sample.Validate(); err != nil {
		err = fmt.Errorf("while validating Sample: %w", err)
		log.Error(err)
		return 0, err
	}

	return len(data), nil
}

// Sum returns the "multihash" of the SampleID.
func (sh *SampleHasher) Sum([]byte) []byte {
	sum, err := sh.sample.ID.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("while marshaling SampleID: %w", err)
		log.Error(err)
	}
	return sum
}

// Reset resets the Hash to its initial state.
func (sh *SampleHasher) Reset() {
	sh.sample = Sample{}
}

// Size returns the number of bytes Sum will return.
func (sh *SampleHasher) Size() int {
	return SampleIDSize
}

// BlockSize returns the hash's underlying block size.
func (sh *SampleHasher) BlockSize() int {
	return sha256.BlockSize
}
