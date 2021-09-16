package block

import (
	"context"
)

// listenForNewBlocks kicks of a listener loop that will
// listen for new "raw" blocks from the Fetcher, and handle
// them.
func (s *Service) listenForNewBlocks(ctx context.Context) error {
	// subscribe to new block events via the block fetcher
	newBlockEventChan, err := s.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		return nil
	}
	// listen for new blocks from channel
	go func() {
		for {
			select {
			case <-s.stopListen:
				return
			case newRawBlock := <- newBlockEventChan:
				s.handleRawBlock(newRawBlock) // TODO @renaynay: how to handle errors here ?

				continue
			}
		}
	}()

	return nil
}

// handleRawBlock handles the new "raw" block, passing it through the
// BlockService pipeline. It first gets erasure coded, then passed to the
//
func (s *Service) handleRawBlock(raw *Raw) error {
	// extend the raw block
	// TODO @renaynay: how to handle erasure encoding error?
	_, err := s.extendBlock(raw)
	if err != nil {
		log.Errorf("error while computing extended data square: %s", err) // TODO @renaynay: or fatal log?
		// TODO @renaynay: panic?
	}
	// TODO @renaynay:
	// generate DAH using extended block
	// verify generated DAH against raw block data root
		// if fraud, generate fraud proof
	// store extended block

	return nil
}