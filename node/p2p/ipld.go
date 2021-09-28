package p2p

import (
	"github.com/ipfs/go-blockservice"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
)

// DAG constructs IPFS's DAG Service for fetching arbitrary Merkle structures.
func DAG(bs blockstore.Blockstore, ex exchange.Interface) format.DAGService {
	return merkledag.NewDAGService(blockservice.New(bs, ex))
}
