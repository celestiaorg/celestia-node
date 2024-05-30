package bitswap

import (
	"encoding"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// DefaultAllowlist keeps default list of multihashes allowed in the network.
// TODO(@Wondertan): Make it private and instead provide Blockservice constructor with injected
// allowlist
var DefaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except registered
	_, ok := specRegistry[code]
	return ok
}

func extractCID(cid cid.Cid) ([]byte, error) {
	if err := validateCID(cid); err != nil {
		return nil, err
	}
	// mhPrefixSize is the size of the multihash prefix that used to cut it off.
	const mhPrefixSize = 4
	return cid.Hash()[mhPrefixSize:], nil
}

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