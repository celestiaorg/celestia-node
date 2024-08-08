package shwap

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"sync"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"
)

// NewBlockService creates a new blockservice.BlockService with allowlist supporting the protocol.
func NewBlockService(b blockstore.Blockstore, ex exchange.Interface) blockservice.BlockService {
	return blockservice.New(b, ex, blockservice.WithAllowlist(defaultAllowlist))
}

var log = logger.Logger("shwap")

const (
	// sampleCodec is a CID codec used for share sampling Bitswap requests over Namespaced
	// Merkle Tree.
	sampleCodec = 0x7800

	// sampleMultihashCode is the multihash code for share sampling multihash function.
	sampleMultihashCode = 0x7801

	// rowCodec is a CID codec used for row Bitswap requests over Namespaced Merkle
	// Tree.
	rowCodec = 0x7810

	// rowMultihashCode is the multihash code for custom axis sampling multihash function.
	rowMultihashCode = 0x7811

	// dataCodec is a CID codec used for data Bitswap requests over Namespaced Merkle Tree.
	dataCodec = 0x7820

	// dataMultihashCode is the multihash code for data multihash function.
	dataMultihashCode = 0x7821

	// mhPrefixSize is the size of the multihash prefix that used to cut it off.
	mhPrefixSize = 4
)

var (
	hashFn = sha256.New
)

func init() {
	// Register hashers for new multihashes
	mh.Register(sampleMultihashCode, func() hash.Hash {
		return &SampleHasher{}
	})
	mh.Register(rowMultihashCode, func() hash.Hash {
		return &RowHasher{}
	})
	mh.Register(dataMultihashCode, func() hash.Hash {
		return &DataHasher{}
	})
}

var (
	rowVerifiers    verifiers[RowID, Row]
	sampleVerifiers verifiers[SampleID, Sample]
	dataVerifiers   verifiers[DataID, Data]
)

type verifiers[ID comparable, V any] struct {
	mp sync.Map
}

func (v *verifiers[ID, V]) Add(id ID, f func(V) error) {
	v.mp.Store(id, f)
}

func (v *verifiers[ID, V]) Verify(id ID, val V) error {
	f, ok := v.mp.LoadAndDelete(id)
	if !ok {
		return fmt.Errorf("no verifier")
	}

	return f.(func(V) error)(val)
}

// defaultAllowlist keeps default list of hashes allowed in the network.
var defaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except home-baked code
	switch code {
	case sampleMultihashCode, rowMultihashCode, dataMultihashCode:
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
	case sampleCodec, rowCodec, dataCodec:
	}

	switch prefix.MhLength {
	default:
		return fmt.Errorf("unsupported multihash length %d", prefix.MhLength)
	case SampleIDSize, RowIDSize, DataIDSize:
	}

	return nil
}
