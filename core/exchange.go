package core

import (
	"bytes"
	"context"
	"fmt"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds"
)

type Exchange struct {
	fetcher   *BlockFetcher
	store     *eds.Store
	construct header.ConstructFn
}

func NewExchange(
	fetcher *BlockFetcher,
	store *eds.Store,
	construct header.ConstructFn,
) *Exchange {
	return &Exchange{
		fetcher:   fetcher,
		store:     store,
		construct: construct,
	}
}

func (ce *Exchange) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	log.Debugw("requesting header", "height", height)
	intHeight := int64(height)
	return ce.getExtendedHeaderByHeight(ctx, &intHeight)
}

func (ce *Exchange) GetRangeByHeight(ctx context.Context, from, amount uint64) ([]*header.ExtendedHeader, error) {
	if amount == 0 {
		return nil, nil
	}

	log.Debugw("requesting headers", "from", from, "to", from+amount)
	headers := make([]*header.ExtendedHeader, amount)
	for i := range headers {
		extHeader, err := ce.GetByHeight(ctx, from+uint64(i))
		if err != nil {
			return nil, err
		}

		headers[i] = extHeader
	}

	return headers, nil
}

func (ce *Exchange) GetVerifiedRange(
	ctx context.Context,
	from *header.ExtendedHeader,
	amount uint64,
) ([]*header.ExtendedHeader, error) {
	headers, err := ce.GetRangeByHeight(ctx, uint64(from.Height())+1, amount)
	if err != nil {
		return nil, err
	}

	for _, h := range headers {
		err := from.Verify(h)
		if err != nil {
			log.Errorw("verifying next header", "last verified height", from.Height(), "err", err)
			return nil, err
		}
		from = h
	}
	return headers, nil
}

func (ce *Exchange) Get(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	log.Debugw("requesting header", "hash", hash.String())
	block, err := ce.fetcher.GetBlockByHash(ctx, hash)
	if err != nil {
		log.Errorw("fetching block by hash", "hash", hash, "err", err)
		return nil, err
	}

	comm, vals, err := ce.fetcher.GetBlockInfo(ctx, &block.Height)
	if err != nil {
		log.Errorw("fetching block info", "height", &block.Height, "err", err)
		return nil, err
	}

	// extend block data
	eds, err := extendBlock(block.Data)
	if err != nil {
		log.Errorw("extending block data", "height", &block.Height, "err", err)
		return nil, err
	}
	// construct extended header
	eh, err := ce.construct(ctx, &block.Header, comm, vals, eds)
	if err != nil {
		log.Errorw("constructing extended header", "height", &block.Height, "err", err)
		return nil, err
	}
	// verify hashes match
	if !bytes.Equal(hash, eh.Hash()) {
		return nil, fmt.Errorf("incorrect hash in header at height %d: expected %x, got %x",
			&block.Height, hash, eh.Hash())
	}
	err = storeEDS(ctx, eh.DAH.Hash(), eds, ce.store)
	if err != nil {
		log.Errorw("storing EDS to eds.Store", "height", &block.Height, "err", err)
		return nil, err
	}
	return eh, nil
}

func (ce *Exchange) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	log.Debug("requesting head")
	return ce.getExtendedHeaderByHeight(ctx, nil)
}

func (ce *Exchange) getExtendedHeaderByHeight(ctx context.Context, height *int64) (*header.ExtendedHeader, error) {
	b, err := ce.fetcher.GetSignedBlock(ctx, height)
	if err != nil {
		log.Errorw("fetching signed block from core", "height", *height)
		return nil, err
	}
	log.Debugw("fetched signed block from core", "height", b.Header.Height)

	// extend block data
	eds, err := extendBlock(b.Data)
	if err != nil {
		log.Errorw("extending block data", "height", b.Header.Height, "err", err)
		return nil, err
	}
	// create extended header
	eh, err := ce.construct(ctx, &b.Header, &b.Commit, &b.ValidatorSet, eds)
	if err != nil {
		log.Errorw("constructing extended header", "height", b.Header.Height, "err", err)
		return nil, err
	}
	err = storeEDS(ctx, eh.DAH.Hash(), eds, ce.store)
	if err != nil {
		log.Errorw("storing EDS to eds.Store", "err", err)
		return nil, err
	}
	return eh, nil
}
