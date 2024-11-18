package bitswap

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/celestiaorg/celestia-app/v3/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var tracer = otel.Tracer("shwap/bitswap")

// Getter implements share.Getter.
type Getter struct {
	exchange  exchange.SessionExchange
	bstore    blockstore.Blockstore
	availWndw pruner.AvailabilityWindow

	availablePool sync.Pool
	archivalPool  sync.Pool

	cancel context.CancelFunc
}

// NewGetter constructs a new Getter.
func NewGetter(
	exchange exchange.SessionExchange,
	bstore blockstore.Blockstore,
	availWndw pruner.AvailabilityWindow,
) *Getter {
	return &Getter{exchange: exchange, bstore: bstore, availWndw: availWndw}
}

// Start kicks off internal fetching sessions.
//
// We keep Bitswap sessions for the whole Getter lifespan:
//   - Sessions retain useful heuristics about peers, like TTFB
//   - Sessions prefer peers that previously served us related content.
//
// So reusing session is expected to improve fetching performance.
//
// There are two sessions for archival and available data, so archival node peers aren't mixed
// with regular full node peers.
func (g *Getter) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel

	var availableID atomic.Uint64
	g.availablePool.New = func() interface{} {
		log.Debugw("creating new available window session", "id", availableID.Load())
		defer availableID.Add(1)
		return g.exchange.NewSession(ctx)
	}

	var archivalID atomic.Uint64
	g.archivalPool.New = func() interface{} {
		log.Debugw("creating new archival session", "id", archivalID.Load())
		defer archivalID.Add(1)
		return g.exchange.NewSession(ctx)
	}
}

// Stop shuts down Getter's internal fetching session.
func (g *Getter) Stop() {
	g.cancel()
}

// GetShares uses [SampleBlock] and [Fetch] to get and verify samples for given coordinates.
// TODO(@Wondertan): Rework API to get coordinates as a single param to make it ergonomic.
func (g *Getter) GetShares(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	rowIdxs, colIdxs []int,
) ([]libshare.Share, error) {
	if len(rowIdxs) != len(colIdxs) {
		return nil, fmt.Errorf("row indecies and col indices must be same length")
	}

	if len(rowIdxs) == 0 {
		return nil, fmt.Errorf("empty coordinates")
	}

	ctx, span := tracer.Start(ctx, "get-shares")
	defer span.End()

	blks := make([]Block, len(rowIdxs))
	for i, rowIdx := range rowIdxs {
		sid, err := NewEmptySampleBlock(hdr.Height(), rowIdx, colIdxs[i], len(hdr.DAH.RowRoots))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "NewEmptySampleBlock")
			return nil, err
		}

		blks[i] = sid
	}

	isArchival := g.isArchival(hdr)
	span.SetAttributes(attribute.Bool("is_archival", isArchival))

	ses := g.session(isArchival)
	defer g.poolSession(ses, isArchival)

	err := Fetch(ctx, g.exchange, hdr.DAH, blks, WithStore(g.bstore), WithFetcher(ses))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Fetch")
		return nil, err
	}

	shares := make([]libshare.Share, len(blks))
	for i, blk := range blks {
		shares[i] = blk.(*SampleBlock).Container.Share
	}

	span.SetStatus(codes.Ok, "")
	return shares, nil
}

// GetShare uses [GetShare] to fetch and verify single share by the given coordinates.
func (g *Getter) GetShare(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	row, col int,
) (libshare.Share, error) {
	shrs, err := g.GetShares(ctx, hdr, []int{row}, []int{col})
	if err != nil {
		return libshare.Share{}, err
	}

	if len(shrs) != 1 {
		return libshare.Share{}, fmt.Errorf("expected 1 share row, got %d", len(shrs))
	}

	return shrs[0], nil
}

// GetEDS uses [RowBlock] and [Fetch] to get half of the first EDS quadrant(ODS) and
// recomputes the whole EDS from it.
// We fetch the ODS or Q1 to ensure better compatibility with archival nodes that only
// store ODS and do not recompute other quadrants.
func (g *Getter) GetEDS(
	ctx context.Context,
	hdr *header.ExtendedHeader,
) (*rsmt2d.ExtendedDataSquare, error) {
	ctx, span := tracer.Start(ctx, "get-eds")
	defer span.End()

	sqrLn := len(hdr.DAH.RowRoots)
	blks := make([]Block, sqrLn/2)
	for i := 0; i < sqrLn/2; i++ {
		blk, err := NewEmptyRowBlock(hdr.Height(), i, sqrLn)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "NewEmptyRowBlock")
			return nil, err
		}

		blks[i] = blk
	}

	isArchival := g.isArchival(hdr)
	span.SetAttributes(attribute.Bool("is_archival", isArchival))

	ses := g.session(isArchival)
	defer g.poolSession(ses, isArchival)

	err := Fetch(ctx, g.exchange, hdr.DAH, blks, WithFetcher(ses))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Fetch")
		return nil, err
	}

	rows := make([]shwap.Row, len(blks))
	for i, blk := range blks {
		rows[i] = blk.(*RowBlock).Container
	}

	square, err := edsFromRows(hdr.DAH, rows)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "edsFromRows")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return square, nil
}

// GetNamespaceData uses [RowNamespaceDataBlock] and [Fetch] to get all the data
// by the given namespace. If data spans over multiple rows, the request is split into
// parallel RowNamespaceDataID requests per each row and then assembled back into NamespaceData.
func (g *Getter) GetNamespaceData(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	ns libshare.Namespace,
) (shwap.NamespaceData, error) {
	if err := ns.ValidateForData(); err != nil {
		return nil, err
	}

	ctx, span := tracer.Start(ctx, "get-shares-by-namespace")
	defer span.End()

	rowIdxs, err := share.RowsWithNamespace(hdr.DAH, ns)
	if err != nil {
		return nil, err
	}
	blks := make([]Block, len(rowIdxs))
	for i, rowNdIdx := range rowIdxs {
		rndblk, err := NewEmptyRowNamespaceDataBlock(hdr.Height(), rowNdIdx, ns, len(hdr.DAH.RowRoots))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "NewEmptyRowNamespaceDataBlock")
			return nil, err
		}
		blks[i] = rndblk
	}

	isArchival := g.isArchival(hdr)
	span.SetAttributes(attribute.Bool("is_archival", isArchival))

	ses := g.session(isArchival)
	defer g.poolSession(ses, isArchival)

	if err = Fetch(ctx, g.exchange, hdr.DAH, blks, WithFetcher(ses)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Fetch")
		return nil, err
	}

	nsShrs := make(shwap.NamespaceData, len(blks))
	for i, blk := range blks {
		rnd := blk.(*RowNamespaceDataBlock).Container
		nsShrs[i] = shwap.RowNamespaceData{
			Shares: rnd.Shares,
			Proof:  rnd.Proof,
		}
	}

	span.SetStatus(codes.Ok, "")
	return nsShrs, nil
}

// isArchival reports whether the header is for archival data
func (g *Getter) isArchival(hdr *header.ExtendedHeader) bool {
	return !pruner.IsWithinAvailabilityWindow(hdr.Time(), g.availWndw)
}

// session takes a session out of the respective session pool
func (g *Getter) session(isArchival bool) exchange.Fetcher {
	if isArchival {
		v := g.archivalPool.Get()
		if v == nil {
			panic("Getter must be started")
		}

		return v.(exchange.Fetcher)
	}

	v := g.availablePool.Get()
	if v == nil {
		panic("Getter must be started")
	}

	return v.(exchange.Fetcher)
}

// poolSession puts session back into the respective session pool
func (g *Getter) poolSession(session exchange.Fetcher, isArchival bool) {
	if isArchival {
		g.archivalPool.Put(session)
	} else {
		g.availablePool.Put(session)
	}
}

// edsFromRows imports given Rows and computes EDS out of them, assuming enough Rows were provided.
// It is designed to reuse Row halves computed during verification on [Fetch] level.
func edsFromRows(roots *share.AxisRoots, rows []shwap.Row) (*rsmt2d.ExtendedDataSquare, error) {
	shrs := make([]libshare.Share, len(roots.RowRoots)*len(roots.RowRoots))
	for i, row := range rows {
		rowShrs, err := row.Shares()
		if err != nil {
			return nil, fmt.Errorf("decoding Shares out of Row: %w", err)
		}

		for j, shr := range rowShrs {
			shrs[j+(i*len(roots.RowRoots))] = shr
		}
	}

	square, err := rsmt2d.ImportExtendedDataSquare(
		libshare.ToBytes(shrs),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(len(roots.RowRoots)/2)),
	)
	if err != nil {
		return nil, fmt.Errorf("importing EDS: %w", err)
	}

	err = square.Repair(roots.RowRoots, roots.ColumnRoots)
	if err != nil {
		return nil, fmt.Errorf("repairing EDS: %w", err)
	}

	return square, nil
}
