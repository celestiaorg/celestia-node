package block

import (
	"context"
)

// Service represents the Block service that can be started / stopped on a `Full` node.
// Service contains 4 main functionalities:
//		1. Fetching "raw" blocks from either Celestia Core or other Celestia Full nodes.
// 		2. Erasure coding the "raw" blocks and producing a DataAvailabilityHeader + verifying the Data root.
// 		3. Storing erasure coded blocks.
// 		4. Serving erasure coded blocks to other `Full` node peers.
type Service struct {
	fetcher Fetcher
}

// NewBlockService creates a new instance of block Service.
func NewBlockService(fetcher Fetcher) *Service {
	return &Service{
		fetcher: fetcher,
	}
}

// Start starts the block Service.
// TODO @renaynay: make sure `Start` eventually has the same signature as `Stop`
func (s *Service) Start(ctx context.Context) (<-chan *Raw, error) {
	// TODO @renaynay: this will eventually be self contained within the block package
	return s.fetcher.SubscribeNewBlockEvent(ctx)
}

// Stop stops the block Service.
func (s *Service) Stop(ctx context.Context) error {
	return s.fetcher.UnsubscribeNewBlockEvent(ctx)
}
