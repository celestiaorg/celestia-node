package shwap

import (
	"crypto/sha256"
	"fmt"

	shwap_pb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// SampleHasher implements hash.Hash interface for Sample.
type SampleHasher struct {
	cid []byte
}

// Write expects a marshaled Sample to validate.
func (h *SampleHasher) Write(data []byte) (int, error) {
	samplepb := &shwap_pb.SampleBlock{}
	if err := samplepb.Unmarshal(data); err != nil {
		err = fmt.Errorf("unmarshaling SampleBlock: %w", err)
		log.Error(err)
		return 0, err
	}
	s, err := SampleFromProto(samplepb)
	if err != nil {
		err = fmt.Errorf("unmarshaling Sample: %w", err)
		log.Error(err)
		return 0, err
	}

	root, err := getRoot(s.SampleID)
	if err != nil {
		err = fmt.Errorf("getting root: %w", err)
		return 0, err
	}

	if err := s.Verify(root); err != nil {
		err = fmt.Errorf("verifying Data: %w", err)
		log.Error(err)
		return 0, err
	}

	const pbOffset = 2
	h.cid = data[pbOffset : SampleIDSize+pbOffset]
	return len(data), nil
}

// Sum returns the "multihash" of the SampleID.
func (h *SampleHasher) Sum([]byte) []byte {
	return h.cid
}

// Reset resets the Hash to its initial state.
func (h *SampleHasher) Reset() {
	h.cid = nil
}

// Size returns the number of bytes Sum will return.
func (h *SampleHasher) Size() int {
	return SampleIDSize
}

// BlockSize returns the hash's underlying block size.
func (h *SampleHasher) BlockSize() int {
	return sha256.BlockSize
}
