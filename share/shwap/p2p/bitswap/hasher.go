package bitswap

import (
	"crypto/sha256"
	"fmt"
)

// hasher
// TODO: Describe the Hack this all is
type hasher struct {
	IDSize int

	sum []byte
}

func (h *hasher) Write(data []byte) (int, error) {
	const pbOffset = 2 // this assumes the protobuf serialization is in use
	if len(data) < h.IDSize+pbOffset {
		err := fmt.Errorf("shwap/bitswap hasher: insufficient data size")
		log.Error()
		return 0, err
	}
	// extract ID out of data
	// we do this on the raw data to:
	//  * Avoid complicating hasher with generalized bytes -> type unmarshalling
	//  * Avoid type allocations
	id := data[pbOffset : h.IDSize+pbOffset]
	// get registered verifier and use it to check data validity
	ver := globalVerifiers.get(string(id))
	if ver == nil {
		err := fmt.Errorf("shwap/bitswap hasher: no verifier registered")
		log.Error(err)
		return 0, err
	}
	err := ver(data)
	if err != nil {
		err = fmt.Errorf("shwap/bitswap hasher: verifying data: %w", err)
		log.Error(err)
		return 0, err
	}
	// if correct set the id as resulting sum
	// it's required for the sum to match the original ID
	// to satisfy hash contract
	h.sum = id
	return len(data), err
}

func (h *hasher) Sum([]byte) []byte {
	return h.sum
}

func (h *hasher) Reset() {
	h.sum = nil
}

func (h *hasher) Size() int {
	return h.IDSize
}

func (h *hasher) BlockSize() int {
	return sha256.BlockSize
}
