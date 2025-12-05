package bitswap

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-app/v6/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var disablePooling = os.Getenv("CELESTIA_BITSWAP_DISABLE_POOLING") == "1"

var tracer = otel.Tracer("shwap/bitswap")

// Getter implements share.Getter.
type Getter struct {
	exchange  exchange.SessionExchange
	bstore    blockstore.Blockstore
	availWndw time.Duration

	availablePool *pool
	archivalPool  *pool

	cancel context.CancelFunc
}

// NewGetter constructs a new Getter.
func NewGetter(
	exchange exchange.SessionExchange,
	bstore blockstore.Blockstore,
	availWndw time.Duration,
) *Getter {
	return &Getter{
		exchange:      exchange,
		bstore:        bstore,
		availWndw:     availWndw,
		availablePool: newPool(exchange),
		archivalPool:  newPool(exchange),
	}
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

	g.availablePool.ctx = ctx
	g.archivalPool.ctx = ctx
}

// Stop shuts down Getter's internal fetching getSession.
func (g *Getter) Stop() {
	g.cancel()
}

// GetSamples uses [SampleBlock] and [Fetch] to get and verify samples for given coordinates.
func (g *Getter) GetSamples(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	indices []shwap.SampleCoords,
) (_ []shwap.Sample, err error) {
	ctx, span := tracer.Start(ctx, "bitswap/get-samples", trace.WithAttributes(
		attribute.Int("amount", len(indices)),
	))
	defer utils.SetStatusAndEnd(span, err)

	if len(indices) == 0 {
		return nil, shwap.ErrNoSampleIndicies
	}

	blks := make([]Block, len(indices))
	for i, idx := range indices {
		sid, err := NewEmptySampleBlock(hdr.Height(), idx, len(hdr.DAH.RowRoots))
		if err != nil {
			return nil, fmt.Errorf("NewEmptySampleBlock: %w", err)
		}

		blks[i] = sid
	}

	isArchival := g.isArchival(hdr)
	span.SetAttributes(attribute.Bool("is_archival", isArchival))

	ses, release := g.getSession(isArchival)
	defer release()

	err = Fetch(ctx, g.exchange, hdr.DAH, blks, WithStore(g.bstore), WithFetcher(ses))

	var fetched int
	smpls := make([]shwap.Sample, len(blks))
	for i, blk := range blks {
		c := blk.(*SampleBlock).Container
		if !c.IsEmpty() {
			fetched++
			smpls[i] = c
		}
	}

	if err != nil {
		if fetched > 0 {
			span.SetAttributes(attribute.Int("fetched", fetched))
			return smpls, err
		}
		return nil, fmt.Errorf("fetched blocks: %w", err)
	}
	return smpls, nil
}

func (g *Getter) GetRow(ctx context.Context, hdr *header.ExtendedHeader, rowIdx int) (_ shwap.Row, err error) {
	ctx, span := tracer.Start(ctx, "bitswap/get-eds")
	defer utils.SetStatusAndEnd(span, err)

	blk, err := NewEmptyRowBlock(hdr.Height(), rowIdx, len(hdr.DAH.RowRoots))
	if err != nil {
		return shwap.Row{}, fmt.Errorf("NewEmptyRowBlock: %w", err)
	}

	isArchival := g.isArchival(hdr)
	span.SetAttributes(attribute.Bool("is_archival", isArchival))

	ses, release := g.getSession(isArchival)
	defer release()

	err = Fetch(ctx, g.exchange, hdr.DAH, []Block{blk}, WithFetcher(ses))
	if err != nil {
		return shwap.Row{}, fmt.Errorf("fetching row: %w", err)
	}
	return blk.Container, nil
}

// GetEDS uses [RowBlock] and [Fetch] to get half of the first EDS quadrant(ODS) and
// recomputes the whole EDS from it.
// We fetch the ODS or Q1 to ensure better compatibility with archival nodes that only
// store ODS and do not recompute other quadrants.
func (g *Getter) GetEDS(
	ctx context.Context,
	hdr *header.ExtendedHeader,
) (_ *rsmt2d.ExtendedDataSquare, err error) {
	ctx, span := tracer.Start(ctx, "bitswap/get-eds")
	defer utils.SetStatusAndEnd(span, err)

	sqrLn := len(hdr.DAH.RowRoots)
	blks := make([]Block, sqrLn/2)
	for i := range sqrLn / 2 {
		blk, err := NewEmptyRowBlock(hdr.Height(), i, sqrLn)
		if err != nil {
			return nil, fmt.Errorf("NewEmptyRowBlock: %w", err)
		}

		blks[i] = blk
	}

	isArchival := g.isArchival(hdr)
	span.SetAttributes(attribute.Bool("is_archival", isArchival))

	ses, release := g.getSession(isArchival)
	defer release()

	err = Fetch(ctx, g.exchange, hdr.DAH, blks, WithFetcher(ses))
	if err != nil {
		return nil, fmt.Errorf("fetching blocks: %w", err)
	}

	rows := make([]shwap.Row, len(blks))
	for i, blk := range blks {
		rows[i] = blk.(*RowBlock).Container
	}

	square, err := edsFromRows(hdr.DAH, rows)
	if err != nil {
		return nil, fmt.Errorf("building eds from rows: %w", err)
	}
	return square, nil
}

// GetNamespaceData uses [RowNamespaceDataBlock] and [Fetch] to get all the data
// by the given namespace. If data spans over multiple rows, the request is split into
// parallel RowNamespaceDataID requests per each row and then assembled back into NamespaceData.
func (g *Getter) GetNamespaceData(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	ns libshare.Namespace,
) (_ shwap.NamespaceData, err error) {
	ctx, span := tracer.Start(ctx, "bitswap/get-shares-by-namespace")
	defer utils.SetStatusAndEnd(span, err)

	if err := ns.ValidateForData(); err != nil {
		return nil, err
	}

	rowIdxs, err := share.RowsWithNamespace(hdr.DAH, ns)
	if err != nil {
		return nil, fmt.Errorf("getting namespace rows: %w", err)
	}
	blks := make([]Block, len(rowIdxs))
	for i, rowNdIdx := range rowIdxs {
		rndblk, err := NewEmptyRowNamespaceDataBlock(hdr.Height(), rowNdIdx, ns, len(hdr.DAH.RowRoots))
		if err != nil {
			return nil, fmt.Errorf("NewEmptyRowNamespaceDataBlock: %w", err)
		}
		blks[i] = rndblk
	}

	isArchival := g.isArchival(hdr)
	span.SetAttributes(attribute.Bool("is_archival", isArchival))

	ses, release := g.getSession(isArchival)
	defer release()

	if err = Fetch(ctx, g.exchange, hdr.DAH, blks, WithFetcher(ses)); err != nil {
		return nil, fmt.Errorf("fetching blocks: %w", err)
	}

	nsShrs := make(shwap.NamespaceData, len(blks))
	for i, blk := range blks {
		rnd := blk.(*RowNamespaceDataBlock).Container
		nsShrs[i] = shwap.RowNamespaceData{
			Shares: rnd.Shares,
			Proof:  rnd.Proof,
		}
	}
	return nsShrs, nil
}

func (g *Getter) GetRangeNamespaceData(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	from, to int,
) (_ shwap.RangeNamespaceData, err error) {
	ctx, span := tracer.Start(ctx, "bitswap/get-range-namespace-data")
	defer utils.SetStatusAndEnd(span, err)

	rangeDataBlock, err := NewEmptyRangeNamespaceDataBlock(
		hdr.Height(), from, to, len(hdr.DAH.RowRoots)/2,
	)
	if err != nil {
		return shwap.RangeNamespaceData{}, fmt.Errorf("NewEmptyRangeNamespaceDataBlock: %w", err)
	}

	isArchival := g.isArchival(hdr)
	span.SetAttributes(attribute.Bool("is_archival", isArchival))

	ses, release := g.getSession(isArchival)
	defer release()

	blks := []Block{rangeDataBlock}

	err = Fetch(ctx, g.exchange, hdr.DAH, blks, WithFetcher(ses))
	if err != nil {
		return shwap.RangeNamespaceData{}, fmt.Errorf("fetching blocks: %w", err)
	}
	return blks[0].(*RangeNamespaceDataBlock).Container, nil
}

// isArchival reports whether the header is for archival data
func (g *Getter) isArchival(hdr *header.ExtendedHeader) bool {
	return !availability.IsWithinWindow(hdr.Time(), g.availWndw)
}

// getSession takes a session out of the respective session pool
func (g *Getter) getSession(isArchival bool) (ses exchange.Fetcher, release func()) {
	if disablePooling {
		ctx, cancel := context.WithCancel(context.Background())
		f := g.exchange.NewSession(ctx)
		return f, cancel
	}

	if isArchival {
		ses = g.archivalPool.get()
		return ses, func() { g.archivalPool.put(ses) }
	}
	ses = g.availablePool.get()
	return ses, func() { g.availablePool.put(ses) }
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

// pool is a pool of Bitswap sessions.
type pool struct {
	lock     sync.Mutex
	sessions []exchange.Fetcher
	ctx      context.Context
	exchange exchange.SessionExchange
}

func newPool(ex exchange.SessionExchange) *pool {
	return &pool{
		exchange: ex,
		sessions: make([]exchange.Fetcher, 0),
	}
}

// get returns a session from the pool or creates a new one if the pool is empty.
func (p *pool) get() exchange.Fetcher {
	p.lock.Lock()
	defer p.lock.Unlock()

	if len(p.sessions) == 0 {
		return p.exchange.NewSession(p.ctx)
	}

	ses := p.sessions[len(p.sessions)-1]
	p.sessions = p.sessions[:len(p.sessions)-1]
	return ses
}

// put returns a session to the pool.
func (p *pool) put(ses exchange.Fetcher) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.sessions = append(p.sessions, ses)
}
