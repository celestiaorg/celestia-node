package ipld

import (
	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
)

// NewBlockservice constructs Blockservice for fetching NMTrees.
func NewBlockservice(bs blockstore.Blockstore, exchange exchange.SessionExchange) blockservice.BlockService {
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
	// we allow all codes except home-baked sha256NamespaceFlagged
	return code == sha256NamespaceFlagged
}
