package p2p

import (
	"github.com/ipfs/go-blockservice"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
)

// blockService constructs IPFS's BlockService for fetching arbitrary Merkle structures.
func blockService(bs blockstore.Blockstore, ex exchange.Interface) blockservice.BlockService {
	return blockservice.New(bs, ex)
}
