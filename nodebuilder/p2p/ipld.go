package p2p

import (
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-blockservice"
)

// blockService constructs IPFS's BlockService for fetching arbitrary Merkle structures.
func blockService(bs blockstore.Blockstore, ex exchange.Interface) blockservice.BlockService {
	return blockservice.New(bs, ex)
}
