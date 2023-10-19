package ipldv2

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/celestia-node/share"
)

var log = logger.Logger("ipldv2")

const (
	// shareSamplingCodec is a CID codec used for share sampling Bitswap requests over Namespaced
	// Merkle Tree.
	shareSamplingCodec = 0x7800

	// shareSamplingMultihashCode is the multihash code for share sampling multihash function.
	shareSamplingMultihashCode = 0x7801

	// axisSamplingCodec is a CID codec used for axis sampling Bitswap requests over Namespaced Merkle
	// Tree.
	axisSamplingCodec = 0x7810

	// axisSamplingMultihashCode is the multihash code for custom axis sampling multihash function.
	axisSamplingMultihashCode = 0x7811
)

// TODO(@Wondertan): Eventually this should become configurable
const (
	hashSize     = sha256.Size
	dahRootSize  = 2*share.NamespaceSize + hashSize
	mhPrefixSize = 4
)

func init() {
	// Register hashers for new multihashes
	mh.Register(shareSamplingMultihashCode, func() hash.Hash {
		return &ShareSampleHasher{}
	})
	mh.Register(axisSamplingMultihashCode, func() hash.Hash {
		return &AxisSampleHasher{}
	})
}

// defaultAllowlist keeps default list of hashes allowed in the network.
var defaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except home-baked code
	switch code {
	case shareSamplingMultihashCode:
	case axisSamplingMultihashCode:
	default:
		return false
	}
	return true
}

func validateCID(cid cid.Cid) error {
	prefix := cid.Prefix()
	if prefix.Codec != shareSamplingCodec && prefix.Codec != axisSamplingCodec {
		return fmt.Errorf("unsupported codec %d", prefix.Codec)
	}

	if prefix.MhType != shareSamplingMultihashCode && prefix.MhType != axisSamplingMultihashCode {
		return fmt.Errorf("unsupported multihash %d", prefix.MhType)
	}

	if prefix.MhLength != ShareSampleIDSize && prefix.MhLength != AxisSampleIDSize {
		return fmt.Errorf("invalid multihash length %d", prefix.MhLength)
	}

	return nil
}

func hashBytes(preimage []byte) []byte {
	hsh := sha256.New()
	hsh.Write(preimage)
	return hsh.Sum(nil)
}
