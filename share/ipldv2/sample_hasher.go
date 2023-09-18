package ipldv2

import (
	"crypto/sha256"
	"hash"

	mh "github.com/multiformats/go-multihash"
)

func init() {
	// Register hasher for multihash.
	mh.Register(multihashCode, func() hash.Hash {
		return &SampleHasher{}
	})
}

// SampleHasher implements hash.Hash interface for Samples.
type SampleHasher struct {
	sample   Sample
}

// Write expects a marshaled Sample to validate.
func (sh *SampleHasher) Write(data []byte) (int, error) {
	err := sh.sample.UnmarshalBinary(data)
	if err != nil {
		log.Error(err)
		return 0, err
	}

	if err = sh.sample.Validate(); err != nil {
		log.Error(err)
		return 0, err
	}

	return len(data), nil
}

// Sum returns the "multihash" of the SampleID.
func (sh *SampleHasher) Sum([]byte) []byte {
	sum, _ := sh.sample.ID.MarshalBinary()
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
