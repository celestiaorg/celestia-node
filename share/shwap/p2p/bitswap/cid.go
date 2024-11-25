package bitswap

import (
	"encoding"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// extractFromCID retrieves Shwap ID out of the CID.
func extractFromCID(cid cid.Cid) ([]byte, error) {
	if err := validateCID(cid); err != nil {
		return nil, fmt.Errorf("invalid cid %s: %w", cid, err)
	}
	// mhPrefixSize is the size of the multihash prefix that used to cut it off.
	const mhPrefixSize = 4
	return cid.Hash()[mhPrefixSize:], nil
}

// encodeToCID encodes Shwap ID into the CID.
func encodeToCID(bm encoding.BinaryMarshaler, mhcode, codec uint64) cid.Cid {
	data, err := bm.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling for CID: %w", err))
	}

	buf, err := mh.Encode(data, mhcode)
	if err != nil {
		panic(fmt.Errorf("encoding to CID: %w", err))
	}

	return cid.NewCidV1(codec, buf)
}

// validateCID checks correctness of the CID.
func validateCID(cid cid.Cid) error {
	spec, err := getSpec(cid)
	if err != nil {
		return err
	}

	prefix := cid.Prefix()
	if prefix.Version != 1 {
		return fmt.Errorf("invalid cid version %d", prefix.Version)
	}
	if prefix.MhType != spec.mhCode {
		return fmt.Errorf("invalid multihash type %d", prefix.MhType)
	}
	if prefix.MhLength != spec.idSize {
		return fmt.Errorf("invalid multihash length %d", prefix.MhLength)
	}

	return nil
}
