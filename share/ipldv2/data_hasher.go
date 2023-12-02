package ipldv2

import (
	"crypto/sha256"
	"fmt"
)

// DataHasher implements hash.Hash interface for Data.
type DataHasher struct {
	data Data
}

// Write expects a marshaled Data to validate.
func (sh *DataHasher) Write(data []byte) (int, error) {
	if err := sh.data.UnmarshalBinary(data); err != nil {
		err = fmt.Errorf("while unmarshaling Data: %w", err)
		log.Error(err)
		return 0, err
	}

	if err := sh.data.Validate(); err != nil {
		err = fmt.Errorf("while validating Data: %w", err)
		log.Error(err)
		return 0, err
	}

	return len(data), nil
}

// Sum returns the "multihash" of the DataID.
func (sh *DataHasher) Sum([]byte) []byte {
	sum, err := sh.data.DataID.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("while marshaling DataID: %w", err)
		log.Error(err)
	}
	return sum
}

// Reset resets the Hash to its initial state.
func (sh *DataHasher) Reset() {
	sh.data = Data{}
}

// Size returns the number of bytes Sum will return.
func (sh *DataHasher) Size() int {
	return DataIDSize
}

// BlockSize returns the hash's underlying block size.
func (sh *DataHasher) BlockSize() int {
	return sha256.BlockSize
}
