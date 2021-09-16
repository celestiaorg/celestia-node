package block

import (
	"context"
	"fmt"
	"sync"

	logging "github.com/ipfs/go-log/v2"
)

// Service represents the Block service that can be started / stopped on a `Full` node.
// Service contains 4 main functionalities:
//		1. Fetching "raw" blocks from either Celestia Core or other Celestia Full nodes.
// 		2. Erasure coding the "raw" blocks and producing a DataAvailabilityHeader + verifying the Data root.
// 		3. Storing erasure coded blocks.
// 		4. Serving erasure coded blocks to other `Full` node peers.
type Service struct {
	fetcher Fetcher

	mux        sync.Mutex
	stopListen chan bool
}

var log = logging.Logger("block-service")

// NewBlockService creates a new instance of block Service.
func NewBlockService(fetcher Fetcher) *Service {
	return &Service{
		fetcher: fetcher,
	}
}

// Start starts the block Service.
// TODO @renaynay: make sure `Start` eventually has the same signature as `Stop`
func (s *Service) Start(ctx context.Context) error {
	if s.stopListen	!= nil {
		return fmt.Errorf("service was already started / not shut down properly") // TODO @renaynay: better err?
	}
	s.mux.Lock()
	s.stopListen = make(chan bool)
	s.mux.Unlock()

	return s.listenForNewBlocks(ctx)
}

// Stop stops the block Service.
func (s *Service) Stop(ctx context.Context) error {
	// send stop signal to listener and shut down
	if s.stopListen == nil {
		return fmt.Errorf("service already stopped / not shut down properly") // TODO @renaynay: better err?
	}
	s.stopListen <- true
	close(s.stopListen)

	s.mux.Lock()
	s.stopListen = nil
	s.mux.Unlock()

	return s.fetcher.UnsubscribeNewBlockEvent(ctx)
}
