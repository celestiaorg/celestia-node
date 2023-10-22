package ipldv2

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"
)

var log = logger.Logger("ipldv2")

const (
	// sampleCodec is a CID codec used for share sampling Bitswap requests over Namespaced
	// Merkle Tree.
	sampleCodec = 0x7800

	// sampleMultihashCode is the multihash code for share sampling multihash function.
	sampleMultihashCode = 0x7801

	// axisCodec is a CID codec used for axis sampling Bitswap requests over Namespaced Merkle
	// Tree.
	axisCodec = 0x7810

	// axisMultihashCode is the multihash code for custom axis sampling multihash function.
	axisMultihashCode = 0x7811

	// mhPrefixSize is the size of the multihash prefix that used to cut it off.
	mhPrefixSize = 4
)

var (
	hashSize = sha256.Size
	hasher   = sha256.New
)

func init() {
	// Register hashers for new multihashes
	mh.Register(sampleMultihashCode, func() hash.Hash {
		return &SampleHasher{}
	})
	mh.Register(axisMultihashCode, func() hash.Hash {
		return &AxisHasher{}
	})
}

// defaultAllowlist keeps default list of hashes allowed in the network.
var defaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except home-baked code
	switch code {
	case sampleMultihashCode:
	case axisMultihashCode:
	default:
		return false
	}
	return true
}

func validateCID(cid cid.Cid) error {
	prefix := cid.Prefix()
	if prefix.Codec != sampleCodec && prefix.Codec != axisCodec {
		return fmt.Errorf("unsupported codec %d", prefix.Codec)
	}

	if prefix.MhType != sampleMultihashCode && prefix.MhType != axisMultihashCode {
		return fmt.Errorf("unsupported multihash %d", prefix.MhType)
	}

	if prefix.MhLength != SampleIDSize && prefix.MhLength != AxisIDSize {
		return fmt.Errorf("invalid multihash length %d", prefix.MhLength)
	}

	return nil
}

func hashBytes(preimage []byte) []byte {
	hsh := hasher()
	hsh.Write(preimage)
	return hsh.Sum(nil)
}
