package block

import (
	"context"
)

// Service represents the Block service that can be started / stopped on a `Full` node.
// Service contains 4 main functionalities:
//		1. Fetching "raw" blocks from either Celestia Core or other Celestia Full nodes. // TODO note: this will
//		 TODO eventually take place on the p2p level.
// 		2. Erasure coding the "raw" blocks and producing a DataAvailabilityHeader + verifying the Data root.
// 		3. Storing erasure coded blocks.
// 		4. Serving erasure coded blocks to other `Full` node peers. // TODO note: optional for devnet
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
func (s *Service) Start(ctx context.Context) (<-chan *Raw, error) {
	return s.fetcher.SubscribeNewBlockEvent(ctx)
}

// Stop stops the block Service.
func (s *Service) Stop(ctx context.Context) error {
	return s.fetcher.UnsubscribeNewBlockEvent(ctx)
}
