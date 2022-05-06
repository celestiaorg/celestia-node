package header

import (
	"bytes"
	"context"
	"fmt"

	format "github.com/ipfs/go-ipld-format"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/core"
)

type CoreExchange struct {
	fetcher    *core.BlockFetcher
	shareStore format.DAGService
}

func NewCoreExchange(fetcher *core.BlockFetcher, dag format.DAGService) *CoreExchange {
	return &CoreExchange{
		fetcher:    fetcher,
		shareStore: dag,
	}
}

func (ce *CoreExchange) RequestHeader(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	log.Debugw("core: requesting header", "height", height)
	intHeight := int64(height)
	return ce.getExtendedHeaderByHeight(ctx, &intHeight)
}

func (ce *CoreExchange) RequestHeaders(ctx context.Context, from, amount uint64) ([]*ExtendedHeader, error) {
	if amount == 0 {
		return nil, nil
	}

	log.Debugw("core: requesting headers", "from", from, "to", from+amount)
	headers := make([]*ExtendedHeader, amount)
	for i := range headers {
		extHeader, err := ce.RequestHeader(ctx, from+uint64(i))
		if err != nil {
			return nil, err
		}

		headers[i] = extHeader
	}

	return headers, nil
}

func (ce *CoreExchange) RequestByHash(ctx context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error) {
	log.Debugw("core: requesting header", "hash", hash.String())
	block, err := ce.fetcher.GetBlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	comm, vals, err := ce.fetcher.GetBlockInfo(ctx, &block.Height)
	if err != nil {
		return nil, err
	}

	eh, err := MakeExtendedHeader(ctx, block, comm, vals, ce.shareStore)
	if err != nil {
		return nil, err
	}

	// verify hashes match
	if !bytes.Equal(hash, eh.Hash()) {
		return nil, fmt.Errorf("incorrect hash in header: expected %x, got %x", hash, eh.Hash())
	}

	return eh, nil
}

func (ce *CoreExchange) RequestHead(ctx context.Context) (*ExtendedHeader, error) {
	log.Debug("core: requesting head")
	return ce.getExtendedHeaderByHeight(ctx, nil)
}

func (ce *CoreExchange) getExtendedHeaderByHeight(ctx context.Context, height *int64) (*ExtendedHeader, error) {
	b, err := ce.fetcher.GetBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	comm, vals, err := ce.fetcher.GetBlockInfo(ctx, &b.Height)
	if err != nil {
		return nil, err
	}

	return MakeExtendedHeader(ctx, b, comm, vals, ce.shareStore)
}
