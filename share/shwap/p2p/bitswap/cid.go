package bitswap

import (
	"encoding"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// extractCID retrieves Shwap ID out of the CID.
func extractCID(cid cid.Cid) ([]byte, error) {
	if err := validateCID(cid); err != nil {
		return nil, err
	}
	// mhPrefixSize is the size of the multihash prefix that used to cut it off.
	const mhPrefixSize = 4
	return cid.Hash()[mhPrefixSize:], nil
}

// encodeCID encodes Shwap ID into the CID.
func encodeCID(bm encoding.BinaryMarshaler, mhcode, codec uint64) cid.Cid {
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
	prefix := cid.Prefix()
	spec, ok := specRegistry[prefix.MhType]
	if !ok {
		return fmt.Errorf("unsupported multihash type %d", prefix.MhType)
	}

	if prefix.Codec != spec.codec {
		return fmt.Errorf("invalid CID codec %d", prefix.Codec)
	}

	if prefix.MhLength != spec.idSize {
		return fmt.Errorf("invalid multihash length %d", prefix.MhLength)
	}

	return nil
}
