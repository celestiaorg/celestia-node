package shwap

import (
	"crypto/sha256"
	"fmt"
)

// SampleHasher implements hash.Hash interface for Sample.
type SampleHasher struct {
	data []byte
}

// Write expects a marshaled Sample to validate.
func (h *SampleHasher) Write(data []byte) (int, error) {
	var s Sample
	if err := s.UnmarshalBinary(data); err != nil {
		err = fmt.Errorf("unmarshaling Sample: %w", err)
		log.Error(err)
		return 0, err
	}

	if err := sampleVerifiers.Verify(s.SampleID, s); err != nil {
		err = fmt.Errorf("verifying Sample: %w", err)
		log.Error(err)
		return 0, err
	}

	h.data = data
	return len(data), nil
}

// Sum returns the "multihash" of the SampleID.
func (h *SampleHasher) Sum([]byte) []byte {
	if h.data == nil {
		return nil
	}
	const pbOffset = 2
	return h.data[pbOffset : SampleIDSize+pbOffset]
}

// Reset resets the Hash to its initial state.
func (h *SampleHasher) Reset() {
	h.data = nil
}

// Size returns the number of bytes Sum will return.
func (h *SampleHasher) Size() int {
	return SampleIDSize
}

// BlockSize returns the hash's underlying block size.
func (h *SampleHasher) BlockSize() int {
	return sha256.BlockSize
}
