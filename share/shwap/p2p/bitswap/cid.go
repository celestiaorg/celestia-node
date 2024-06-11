package bitswap

import (
	"encoding"
	"encoding/binary"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// DefaultAllowlist keeps default list of multihashes allowed in the network.
// TODO(@Wondertan): Make it private and instead provide Blockservice constructor with injected
//
//	allowlist
var DefaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except registered
	_, ok := specRegistry[code]
	return ok
}

// readCID reads out cid out of  bytes
func readCID(data []byte) (cid.Cid, error) {
	cidLen, ln := binary.Uvarint(data)
	if ln <= 0 || len(data) < ln+int(cidLen) {
		return cid.Undef, fmt.Errorf("invalid data length")
	}
	// extract CID out of data
	// we do this on the raw data to:
	//  * Avoid complicating hasher with generalized bytes -> type unmarshalling
	//  * Avoid type allocations
	cidRaw := data[ln : ln+int(cidLen)]
	castCid, err := cid.Cast(cidRaw)
	if err != nil {
		return cid.Undef, fmt.Errorf("casting cid: %w", err)
	}

	return castCid, nil
}

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

	if prefix.MhLength != spec.size {
		return fmt.Errorf("invalid multihash length %d", prefix.MhLength)
	}

	return nil
}
