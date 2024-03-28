package shwap

import (
	"crypto/sha256"
	"fmt"
)

// RowHasher implements hash.Hash interface for Row.
type RowHasher struct {
	data []byte
}

// Write expects a marshaled Row to validate.
func (h *RowHasher) Write(data []byte) (int, error) {
	var row Row
	if err := row.UnmarshalBinary(data); err != nil {
		err = fmt.Errorf("unmarshaling Row: %w", err)
		log.Error(err)
		return 0, err
	}

	if err := rowVerifiers.Verify(row.RowID, row); err != nil {
		err = fmt.Errorf("verifying Row: %w", err)
		log.Error(err)
		return 0, err
	}

	h.data = data
	return len(data), nil
}

// Sum returns the "multihash" of the RowID.
func (h *RowHasher) Sum([]byte) []byte {
	if h.data == nil {
		return nil
	}
	const pbOffset = 2
	return h.data[pbOffset : RowIDSize+pbOffset]
}

// Reset resets the Hash to its initial state.
func (h *RowHasher) Reset() {
	h.data = nil
}

// Size returns the number of bytes Sum will return.
func (h *RowHasher) Size() int {
	return RowIDSize
}

// BlockSize returns the hash's underlying block size.
func (h *RowHasher) BlockSize() int {
	return sha256.BlockSize
}
