package block

import (
	"context"

	"github.com/celestiaorg/celestia-node/service/header"
)

// listenForNewBlocks kicks of a listener loop that will
// listen for new "raw" blocks from the Fetcher, and handle
// them.
func (s *Service) listenForNewBlocks(ctx context.Context) error {
	// subscribe to new block events via the block fetcher
	newBlockEventChan, err := s.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		return err
	}
	log.Debug("subscribed to new block events")
	// listen for new blocks from channel
	go func() {
		for {
			newRawBlock, ok := <-newBlockEventChan
			if !ok {
				return
			}
			log.Infow("received new block", "block height", newRawBlock.Height, "block hash",
				newRawBlock.Hash())
			err = s.handleRawBlock(newRawBlock)
			if err != nil {
				log.Errorw("handling raw block", "err msg", err, "block height",
					newRawBlock.Height, "block hash", newRawBlock.Hash())
			}
		}
	}()

	return nil
}

// handleRawBlock handles the new "raw" block, passing it through the
// BlockService 4-step pipeline:
// 1. Raw block data gets erasure coded.
// 2. A DataAvailabilityHeader is created from the erasure coded block data.
// 3. A fraud proof is generated based on whether the generated DataAvailabilityHeader's
//   root matches the one in the header of the raw block.
// 4. The Block gets stored.
func (s *Service) handleRawBlock(raw *RawBlock) error {
	// extend the raw block
	extendedBlockData, err := extendBlockData(raw)
	if err != nil {
		log.Errorw("computing extended data square", "err msg", err, "block height", raw.Height,
			"block hash", raw.Hash().String())
		return err
	}
	// generate DAH using extended block
	_, err = header.DataAvailabilityHeaderFromExtendedData(extendedBlockData)
	if err != nil {
		log.Errorw("computing DataAvailabilityHeader", "err msg", err, "block height", raw.Height,
			"block hash", raw.Hash().String())
		return err
	}
	// create Block
	extendedBlock := &Block{
		data: extendedBlockData,
	}
	// check for bad encoding fraud
	err = s.validateEncoding(extendedBlock, raw.Header)
	if err != nil {
		log.Errorw("checking for bad encoding", "err msg", err, "block height", raw.Height,
			"block hash", raw.Hash().String())
		return err
	}
	// TODO @renaynay: store extended block
	return nil
}
