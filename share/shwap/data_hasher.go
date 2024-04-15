package shwap

import (
	"crypto/sha256"
	"fmt"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// DataHasher implements hash.Hash interface for Data.
type DataHasher struct {
	data []byte
}

// Write expects a marshaled Data to validate.
func (h *DataHasher) Write(data []byte) (int, error) {
	datapb := &shwappb.DataBlock{}
	if err := datapb.Unmarshal(data); err != nil {
		err = fmt.Errorf("unmarshaling DataBlock: %w", err)
		log.Error(err)
		return 0, err
	}

	d, err := DataFromProto(datapb)
	if err != nil {
		err = fmt.Errorf("unmarshaling Data: %w", err)
		log.Error(err)
		return 0, err
	}

	root, err := getRoot(d.DataID)
	if err != nil {
		err = fmt.Errorf("getting root: %w", err)
		return 0, err
	}

	if err := d.Verify(root); err != nil {
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
