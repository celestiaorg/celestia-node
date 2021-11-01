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
				log.Debug("new block event channel closed")
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
	dah, err := header.DataAvailabilityHeaderFromExtendedData(extendedBlockData)
	if err != nil {
		log.Errorw("computing DataAvailabilityHeader", "err msg", err, "block height", raw.Height,
			"block hash", raw.Hash().String())
		return err
	}
	log.Debugw("generated DataAvailabilityHeader", "data root", dah.Hash())
	// create ExtendedHeader
	commit, err := s.fetcher.CommitAtHeight(context.Background(), &raw.Height)
	if err != nil {
		log.Errorw("fetching commit", "err", err.Error(), "height", raw.Height)
		return err
	}
	extendedHeader := &header.ExtendedHeader{
		RawHeader: raw.Header,
		DAH:       &dah,
		Commit:    commit,
		// TODO @renaynay: validator set
	}
	// create Block
	extendedBlock := &Block{
		header: extendedHeader,
		data:   extendedBlockData,
	}
	// check for bad encoding fraud
	err = validateEncoding(extendedBlock, raw.Header)
	if err != nil {
		log.Errorw("checking for bad encoding", "err msg", err, "block height", raw.Height,
			"block hash", raw.Hash().String())
		return err
	}
	err = s.StoreBlockData(context.Background(), extendedBlockData)
	if err != nil {
		log.Errorw("storing block", "err msg", err, "block height", raw.Height, "block hash",
			raw.Hash().String())
		return err
	}
	log.Infow("stored block", "block height", raw.Height, "block hash", raw.Hash().String())
	return nil
}
