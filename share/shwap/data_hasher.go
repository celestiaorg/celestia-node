package shwap

import (
	"crypto/sha256"
	"fmt"
)

// DataHasher implements hash.Hash interface for Data.
type DataHasher struct {
	data []byte
}

// Write expects a marshaled Data to validate.
func (h *DataHasher) Write(data []byte) (int, error) {
	var d Data
	if err := d.UnmarshalBinary(data); err != nil {
		err = fmt.Errorf("unmarshaling Data: %w", err)
		log.Error(err)
		return 0, err
	}

	if err := dataVerifiers.Verify(d.DataID, d); err != nil {
		err = fmt.Errorf("verifying Data: %w", err)
		log.Error(err)
		return 0, err
	}

	h.data = data
	return len(data), nil
}

// Sum returns the "multihash" of the DataID.
func (h *DataHasher) Sum([]byte) []byte {
	if h.data == nil {
		return nil
	}
	const pbOffset = 2
	return h.data[pbOffset : DataIDSize+pbOffset]
}

// Reset resets the Hash to its initial state.
func (h *DataHasher) Reset() {
	h.data = nil
}

// Size returns the number of bytes Sum will return.
func (h *DataHasher) Size() int {
	return DataIDSize
}

// BlockSize returns the hash's underlying block size.
func (h *DataHasher) BlockSize() int {
	return sha256.BlockSize
}
