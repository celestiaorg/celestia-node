package core

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ipfs/go-blockservice"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
)

type Exchange struct {
	fetcher    *BlockFetcher
	shareStore blockservice.BlockService
	construct  header.ConstructFn
}

func NewExchange(
	fetcher *BlockFetcher,
	bServ blockservice.BlockService,
	construct header.ConstructFn,
) *Exchange {
	return &Exchange{
		fetcher:    fetcher,
		shareStore: bServ,
		construct:  construct,
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
		err := from.VerifyAdjacent(h)
		if err != nil {
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
		return nil, err
	}

	comm, vals, err := ce.fetcher.GetBlockInfo(ctx, &block.Height)
	if err != nil {
		return nil, err
	}

	eh, err := ce.construct(ctx, block, comm, vals, ce.shareStore)
	if err != nil {
		return nil, err
	}

	// verify hashes match
	if !bytes.Equal(hash, eh.Hash()) {
		return nil, fmt.Errorf("incorrect hash in header: expected %x, got %x", hash, eh.Hash())
	}

	return eh, nil
}

func (ce *Exchange) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	log.Debug("requesting head")
	return ce.getExtendedHeaderByHeight(ctx, nil)
}

func (ce *Exchange) getExtendedHeaderByHeight(ctx context.Context, height *int64) (*header.ExtendedHeader, error) {
	b, err := ce.fetcher.GetBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	comm, vals, err := ce.fetcher.GetBlockInfo(ctx, &b.Height)
	if err != nil {
		return nil, err
	}

	return ce.construct(ctx, b, comm, vals, ce.shareStore)
}
