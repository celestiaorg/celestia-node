package ipldv2

import (
	"crypto/sha256"
	"fmt"
)

// ShareSampleHasher implements hash.Hash interface for Samples.
type ShareSampleHasher struct {
	sample ShareSample
}

// Write expects a marshaled ShareSample to validate.
func (sh *ShareSampleHasher) Write(data []byte) (int, error) {
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
func (sh *ShareSampleHasher) Sum([]byte) []byte {
	sum, err := sh.sample.ID.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("while marshaling ShareSampleID")
		log.Error(err)
	}
	return sum
}

// Reset resets the Hash to its initial state.
func (sh *ShareSampleHasher) Reset() {
	sh.sample = ShareSample{}
}

// Size returns the number of bytes Sum will return.
func (sh *ShareSampleHasher) Size() int {
	return ShareSampleIDSize
}

// BlockSize returns the hash's underlying block size.
func (sh *ShareSampleHasher) BlockSize() int {
	return sha256.BlockSize
}
