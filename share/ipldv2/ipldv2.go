package ipldv2

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// MustSampleCID constructs a sample CID or panics.
func MustSampleCID(axisIdx int, root *share.Root, height uint64) cid.Cid {
	axisTp := rsmt2d.Row // TODO: Randomize axis type
	cid, err := NewSampleID(axisTp, axisIdx, root, height).Cid()
	if err != nil {
		panic("failed to create sample CID")
	}
	return cid
}

// MustAxisCID constructs an axis CID or panics.
func MustAxisCID(axisTp rsmt2d.Axis, axisIdx int, root *share.Root, height uint64) cid.Cid {
	cid, err := NewAxisID(axisTp, uint16(axisIdx), root, height).Cid()
	if err != nil {
		panic("failed to create axis CID")
	}
	return cid
}

// MustDataCID constructs a data CID or panics.
func MustDataCID(axisIdx int, root *share.Root, height uint64, namespace share.Namespace) cid.Cid {
	cid, err := NewDataID(axisIdx, root, height, namespace).Cid()
	if err != nil {
		panic("failed to create data CID")
	}
	return cid
}

// NewBlockService creates a new blockservice.BlockService with allowlist supporting the protocol.
func NewBlockService(b blockstore.Blockstore, ex exchange.Interface) blockservice.BlockService {
	return blockservice.New(b, ex, blockservice.WithAllowlist(defaultAllowlist))
}

var log = logger.Logger("ipldv2")

const (
	// sampleCodec is a CID codec used for share sampling Bitswap requests over Namespaced
	// Merkle Tree.
	sampleCodec = 0x7800

	// sampleMultihashCode is the multihash code for share sampling multihash function.
	sampleMultihashCode = 0x7801

	// axisCodec is a CID codec used for axis Bitswap requests over Namespaced Merkle
	// Tree.
	axisCodec = 0x7810

	// axisMultihashCode is the multihash code for custom axis sampling multihash function.
	axisMultihashCode = 0x7811

	// dataCodec is a CID codec used for data Bitswap requests over Namespaced Merkle Tree.
	dataCodec = 0x7820

	// dataMultihashCode is the multihash code for data multihash function.
	dataMultihashCode = 0x7821

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
	mh.Register(dataMultihashCode, func() hash.Hash {
		return &DataHasher{}
	})
}

// defaultAllowlist keeps default list of hashes allowed in the network.
var defaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except home-baked code
	switch code {
	case sampleMultihashCode, axisMultihashCode, dataMultihashCode:
		return true
	}
	return false
}

func validateCID(cid cid.Cid) error {
	prefix := cid.Prefix()
	if !defaultAllowlist.IsAllowed(prefix.MhType) {
		return fmt.Errorf("unsupported multihash type %d", prefix.MhType)
	}

	switch prefix.Codec {
	default:
		return fmt.Errorf("unsupported codec %d", prefix.Codec)
	case sampleCodec, axisCodec, dataCodec:
	}

	switch prefix.MhLength {
	default:
		return fmt.Errorf("unsupported multihash length %d", prefix.MhLength)
	case SampleIDSize, AxisIDSize, DataIDSize:
	}

	return nil
}

func hashBytes(preimage []byte) []byte {
	hsh := hasher()
	hsh.Write(preimage)
	return hsh.Sum(nil)
}
