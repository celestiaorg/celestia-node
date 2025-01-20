package core

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/types"
	"golang.org/x/sync/errgroup"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/store"
)

const concurrencyLimit = 16

type Exchange struct {
	fetcher   *BlockFetcher
	store     *store.Store
	construct header.ConstructFn

	availabilityWindow time.Duration
	archival           bool

	metrics *exchangeMetrics
}

func NewExchange(
	fetcher *BlockFetcher,
	store *store.Store,
	construct header.ConstructFn,
	opts ...Option,
) (*Exchange, error) {
	p := defaultParams()
	for _, opt := range opts {
		opt(&p)
	}

	var (
		metrics *exchangeMetrics
		err     error
	)
	if p.metrics {
		metrics, err = newExchangeMetrics()
		if err != nil {
			return nil, err
		}
	}

	return &Exchange{
		fetcher:            fetcher,
		store:              store,
		construct:          construct,
		availabilityWindow: p.availabilityWindow,
		archival:           p.archival,
		metrics:            metrics,
	}, nil
}

func (ce *Exchange) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	log.Debugw("requesting header", "height", height)
	intHeight := int64(height)
	return ce.getExtendedHeaderByHeight(ctx, &intHeight)
}

func (ce *Exchange) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	start := time.Now()

	amount := to - (from.Height() + 1)
	headers, err := ce.getRangeByHeight(ctx, from.Height()+1, amount)
	if err != nil {
		return nil, err
	}

	ce.metrics.requestDurationPerHeader(ctx, time.Since(start), amount)

	for _, h := range headers {
		err := libhead.Verify[*header.ExtendedHeader](from, h)
		if err != nil {
			return nil, fmt.Errorf("verifying next header against last verified height: %d: %w",
				from.Height(), err)
		}
		from = h
	}
	return headers, nil
}

func (ce *Exchange) getRangeByHeight(ctx context.Context, from, amount uint64) ([]*header.ExtendedHeader, error) {
	if amount == 0 {
		return nil, nil
	}

	log.Debugw("requesting headers", "from", from, "to", from+amount)
	headers := make([]*header.ExtendedHeader, amount)

	start := time.Now()
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(concurrencyLimit)
	for i := range headers {
		errGroup.Go(func() error {
			extHeader, err := ce.GetByHeight(ctx, from+uint64(i))
			if err != nil {
				return err
			}

			headers[i] = extHeader
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, err
	}
	log.Debugw("received headers", "from", from, "to", from+amount, "after", time.Since(start))
	return headers, nil
}

func (ce *Exchange) Get(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	log.Debugw("requesting header", "hash", hash.String())
	block, err := ce.fetcher.GetBlockByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("fetching block by hash %s: %w", hash.String(), err)
	}

	comm, vals, err := ce.fetcher.GetBlockInfo(ctx, &block.Height)
	if err != nil {
		return nil, fmt.Errorf("fetching block info for height %d: %w", &block.Height, err)
	}

	eds, err := extendBlock(block.Data, block.Header.Version.App)
	if err != nil {
		return nil, fmt.Errorf("extending block data for height %d: %w", &block.Height, err)
	}
	// construct extended header
	eh, err := ce.construct(&block.Header, comm, vals, eds)
	if err != nil {
		panic(fmt.Errorf("constructing extended header for height %d: %w", &block.Height, err))
	}
	// verify hashes match
	if !bytes.Equal(hash, eh.Hash()) {
		return nil, fmt.Errorf("incorrect hash in header at height %d: expected %x, got %x",
			&block.Height, hash, eh.Hash())
	}

	err = storeEDS(ctx, eh, eds, ce.store, ce.availabilityWindow, ce.archival)
	if err != nil {
		return nil, err
	}

	return eh, nil
}

func (ce *Exchange) Head(
	ctx context.Context,
	_ ...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	log.Debug("requesting head")
	return ce.getExtendedHeaderByHeight(ctx, nil)
}

func (ce *Exchange) getExtendedHeaderByHeight(ctx context.Context, height *int64) (*header.ExtendedHeader, error) {
	b, err := ce.fetcher.GetSignedBlock(ctx, height)
	if err != nil {
		if height == nil {
			return nil, fmt.Errorf("fetching signed block for head from core: %w", err)
		}
		return nil, fmt.Errorf("fetching signed block at height %d from core: %w", *height, err)
	}
	log.Debugw("fetched signed block from core", "height", b.Header.Height)

	eds, err := extendBlock(b.Data, b.Header.Version.App)
	if err != nil {
		return nil, fmt.Errorf("extending block data for height %d: %w", b.Header.Height, err)
	}

	// TODO(@Wondertan): This is a hack to deref Data, allowing GC to pick it up.
	//  The better footgun-less solution is to change core.ResultSignedBlock fields to be pointers instead of values.
	b.Data = types.Data{}

	eh, err := ce.construct(&b.Header, &b.Commit, &b.ValidatorSet, eds)
	if err != nil {
		panic(fmt.Errorf("constructing extended header for height %d: %w", b.Header.Height, err))
	}

	err = storeEDS(ctx, eh, eds, ce.store, ce.availabilityWindow, ce.archival)
	if err != nil {
		return nil, err
	}

	return eh, nil
}
