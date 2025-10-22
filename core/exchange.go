package core

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-app/v6/pkg/da"
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

	var metrics *exchangeMetrics
	if p.metrics {
		// set metrics for fetcher
		fetcherMetrics, err := newFetcherMetrics()
		if err != nil {
			return nil, err
		}
		fetcher.metrics = fetcherMetrics

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
	return ce.getExtendedHeaderByHeight(ctx, int64(height))
}

func (ce *Exchange) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	amount := to - (from.Height() + 1)
	headers, err := ce.getRangeByHeight(ctx, from.Height()+1, amount)
	if err != nil {
		return nil, err
	}

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

	comm, vals, err := ce.fetcher.GetBlockInfo(ctx, block.Height)
	if err != nil {
		return nil, fmt.Errorf("fetching block info for height %d: %w", &block.Height, err)
	}

	eds, err := da.ConstructEDS(block.Txs.ToSliceOfBytes(), block.Version.App, -1)
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
	return ce.getExtendedHeaderByHeight(ctx, 0)
}

func (ce *Exchange) getExtendedHeaderByHeight(ctx context.Context, height int64) (*header.ExtendedHeader, error) {
	start := time.Now()
	b, err := ce.fetcher.GetSignedBlock(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("fetching signed block at height %d from core: %w", height, err)
	}
	downloadTime := time.Since(start)
	log.Debugw("fetched signed block from core", "height", b.Header.Height)

	start = time.Now()
	eds, err := da.ConstructEDS(b.Data.Txs.ToSliceOfBytes(), b.Header.Version.App, -1)
	if err != nil {
		return nil, fmt.Errorf("extending block data for height %d: %w", b.Header.Height, err)
	}

	// create extended header
	eh, err := ce.construct(b.Header, b.Commit, b.ValidatorSet, eds)
	if err != nil {
		panic(fmt.Errorf("constructing extended header for height %d: %w", b.Header.Height, err))
	}
	constructTime := time.Since(start)

	start = time.Now()
	err = storeEDS(ctx, eh, eds, ce.store, ce.availabilityWindow, ce.archival)
	if err != nil {
		return nil, err
	}
	storeTime := time.Since(start)

	ce.metrics.observeBlockDownload(ctx, downloadTime, eh.DAH.SquareSize())
	ce.metrics.observeEDSConstruction(ctx, constructTime, eh.DAH.SquareSize())
	ce.metrics.observeEDSStorage(ctx, storeTime, eh.DAH.SquareSize())

	return eh, nil
}
