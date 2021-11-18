package block

import (
	"context"

	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/service/header"
)

// Service represents the Block service that can be started / stopped on a `Full` node.
// Service contains 4 main functionalities:
//		1. Fetching "raw" blocks from either Celestia Core or other Celestia Full nodes.
// 		2. Erasure coding the "raw" blocks and producing a DataAvailabilityHeader + verifying the Data root.
// 		3. Storing erasure coded blocks.
// 		4. Serving erasure coded blocks to other `Full` node peers.
// TODO @renaynay: add docs about broadcaster
type Service struct {
	fetcher     Fetcher
	store       ipld.DAGService
	broadcaster header.Broadcaster
}

var log = logging.Logger("block-service")

// NewBlockService creates a new instance of block Service.
func NewBlockService(fetcher Fetcher, store ipld.DAGService, broadcaster header.Broadcaster) *Service {
	return &Service{
		fetcher:     fetcher,
		store:       store,
		broadcaster: broadcaster,
	}
}

// Start starts the block Service.
func (s *Service) Start(ctx context.Context) error {
	log.Info("starting block service")
	return s.listenForNewBlocks(ctx)
}

// Stop stops the block Service.
func (s *Service) Stop(ctx context.Context) error {
	log.Info("stopping block service")
	// calling unsubscribe will close the newBlockEventCh channel,
	// stopping the listener.
	return s.fetcher.UnsubscribeNewBlockEvent(ctx)
}
