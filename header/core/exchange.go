package core

import (
	"bytes"
	"context"
	"fmt"

	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
)

var log = logging.Logger("header/core")

type Exchange struct {
	fetcher    *core.BlockFetcher
	shareStore format.DAGService
	construct  header.ConstructFn
}

func NewExchange(fetcher *core.BlockFetcher, dag format.DAGService, construct header.ConstructFn) *Exchange {
	return &Exchange{
		fetcher:    fetcher,
		shareStore: dag,
		construct:  construct,
	}
}

func (ce *Exchange) RequestHeader(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	log.Debugw("requesting header", "height", height)
	intHeight := int64(height)
	return ce.getExtendedHeaderByHeight(ctx, &intHeight)
}

func (ce *Exchange) RequestHeaders(ctx context.Context, from, amount uint64) ([]*header.ExtendedHeader, error) {
	if amount == 0 {
		return nil, nil
	}

	log.Debugw("requesting headers", "from", from, "to", from+amount)
	headers := make([]*header.ExtendedHeader, amount)
	for i := range headers {
		extHeader, err := ce.RequestHeader(ctx, from+uint64(i))
		if err != nil {
			return nil, err
		}

		headers[i] = extHeader
	}

	return headers, nil
}

func (ce *Exchange) RequestByHash(ctx context.Context, hash tmbytes.HexBytes) (*header.ExtendedHeader, error) {
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

func (ce *Exchange) RequestHead(ctx context.Context) (*header.ExtendedHeader, error) {
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
