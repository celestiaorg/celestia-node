package core

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-app/v6/pkg/da"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/store"
)

const concurrencyLimit = 16

var tracer = otel.Tracer("core")

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
	var err error

	ctx, span := tracer.Start(ctx, "exchange/getExtendedHeaderByHeight")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	span.SetAttributes(attribute.Int64("height", height))

	b, err := ce.fetcher.GetSignedBlock(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("fetching signed block at height %d from core: %w", height, err)
	}
	span.AddEvent("fetched signed block from core")
	log.Debugw("fetched signed block from core", "height", b.Header.Height)

	eds, err := da.ConstructEDS(b.Data.Txs.ToSliceOfBytes(), b.Header.Version.App, -1)
	if err != nil {
		return nil, fmt.Errorf("extending block data for height %d: %w", b.Header.Height, err)
	}

	// create extended header
	eh, err := ce.construct(b.Header, b.Commit, b.ValidatorSet, eds)
	if err != nil {
		panic(fmt.Errorf("constructing extended header for height %d: %w", b.Header.Height, err))
	}
	span.AddEvent("exchange: constructed extended header",
		trace.WithAttributes(attribute.Int("square_size", eh.DAH.SquareSize())),
	)

	err = storeEDS(ctx, eh, eds, ce.store, ce.availabilityWindow, ce.archival)
	if err != nil {
		return nil, err
	}
	span.AddEvent("exchange: stored square")

	ce.metrics.observeBlockProcessed(ctx, eh.DAH.SquareSize())
	return eh, nil
}
