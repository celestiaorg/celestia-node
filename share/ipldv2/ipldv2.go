package ipldv2

import (
	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
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

// NewBlockservice constructs Blockservice for fetching NMTrees.
func NewBlockservice(bs blockstore.Blockstore, exchange exchange.Interface) blockservice.BlockService {
	return blockservice.New(bs, exchange, blockservice.WithAllowlist(defaultAllowlist))
}

// NewMemBlockservice constructs Blockservice for fetching NMTrees with in-memory blockstore.
func NewMemBlockservice() blockservice.BlockService {
	bstore := blockstore.NewBlockstore(sync.MutexWrap(datastore.NewMapDatastore()))
	return NewBlockservice(bstore, nil)
}

// defaultAllowlist keeps default list of hashes allowed in the network.
var defaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except home-baked code
	return code == multihashCode
}
