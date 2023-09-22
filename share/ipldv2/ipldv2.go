package ipldv2

import (
	"fmt"

	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"
)

var log = logger.Logger("ipldv2")

const (
	// codec is the codec used for leaf and inner nodes of a Namespaced Merkle Tree.
	codec = 0x7800

	// multihashCode is the multihash code used to hash blocks
	// that contain an NMT node (inner and leaf nodes).
	multihashCode = 0x7801
)

// defaultAllowlist keeps default list of hashes allowed in the network.
var defaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except home-baked code
	return code == multihashCode
}

func validateCID(cid cid.Cid) error {
	prefix := cid.Prefix()
	if prefix.Codec != codec {
		return fmt.Errorf("unsupported codec")
	}

	if prefix.MhType != multihashCode {
		return fmt.Errorf("unsupported multihash")
	}

	if prefix.MhLength != SampleIDSize {
		return fmt.Errorf("invalid multihash length")
	}

	return nil
}
