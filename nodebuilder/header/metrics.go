// This file contains the definition of blackbox metrics for the header pkg under nodebuilder
// We adopt as a pattern the definition of a metrics.go file under nodebuidler/<pkg>/metrics.go
// to be the definition of the blackbox metrics only.
// Whitebox metrics that deal with the internals should be defined in the original package
// and not under nodebuilder/<pkg>/metrics.go
package header

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/utils/misc"
)

var (
	meter = global.MeterProvider().Meter("blackbox-header")
)

// blackBoxInstrument is the proxy struct
// used to perform measurements of blackbox metrics
// for the header.Module interface (i.e: the header service)
// check <insert documentation file here> for more info.
type blackBoxInstrument struct {
	// metrics
	requestsNum     syncint64.Counter
	requestDuration syncint64.Histogram
	requestSize     syncint64.Histogram
	blockTime       syncint64.Histogram

	lastHeadTS time.Time
	lastHead   int64

	// pointer to mod
	next Module
}

// constructor that returns a proxied
// interface for blackbox metrics
func newBlackBoxInstrument(next Module) (Module, error) {
	requestsNum, err := meter.
		SyncInt64().
		Counter(
			"node.header.blackbox.requests_count",
			instrument.WithDescription("get height requests count"),
		)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.
		SyncInt64().
		Histogram(
			"node.header.blackbox.request_duration",
			instrument.WithDescription("duration of a single get header request"),
		)
	if err != nil {
		return nil, err
	}

	requestSize, err := meter.
		SyncInt64().
		Histogram(
			"node.header.blackbox.request_size",
			instrument.WithDescription("size of a get header response"),
		)
	if err != nil {
		return nil, err
	}

	blockTime, err := meter.
		SyncInt64().
		Histogram(
			"node.header.blackbox.block_time",
			instrument.WithDescription("test block time"),
		)
	if err != nil {
		return nil, err
	}

	lastHeadTS := time.Now()
	lastHead := int64(0)

	bbinstrument := &blackBoxInstrument{
		requestsNum,
		requestDuration,
		requestSize,
		blockTime,
		lastHeadTS,
		lastHead,
		next,
	}

	return bbinstrument, nil
}

// GetByHeight returns the ExtendedHeader at the given height, blocking
// until header has been processed by the store or context deadline is exceeded.
func (bbi *blackBoxInstrument) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	now := time.Now()
	requestID, err := misc.RandString(5)
	if err != nil {
		return nil, err
	}

	// defer recording the duration until the request has received a response and finished
	defer func(ctx context.Context, begin time.Time) {
		bbi.requestDuration.Record(
			ctx,
			time.Since(begin).Milliseconds(),
		)
	}(ctx, now)

	// perform the actual request
	eh, err := bbi.next.GetByHeight(ctx, height)
	if err != nil {
		// count the request and tag it as a failed one
		bbi.requestsNum.Add(
			ctx,
			1,
			attribute.String("request-id", requestID),
			attribute.String("state", "failed"),
		)
		return eh, err
	}

	// other wise, count the request but tag it as a succeeded one
	bbi.requestsNum.Add(
		ctx,
		1,
		attribute.String("request-id", requestID),
		attribute.String("state", "succeeded"),
	)

	// retrieve the binary format to get the size of the header
	// TODO(@team): is ExtendedHeader.MarshalBinary() == ResponseSize?
	// I am making this assumption for now
	bin, err := eh.MarshalBinary()
	if err != nil {
		return nil, err
	}

	// record the response size (extended header in this case)
	bbi.requestSize.Record(
		ctx,
		int64(len(bin)),
	)

	return eh, err
}

// Head returns the ExtendedHeader of the chain head.
func (bbi *blackBoxInstrument) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	return bbi.next.Head(ctx)
}

// Head returns the ExtendedHeader of the chain head.
func (bbi *blackBoxInstrument) SyncerHead(ctx context.Context) (*header.ExtendedHeader, error) {
	header, err := bbi.next.Head(ctx)
	if err != nil {
		return nil, err
	}

	if header.RawHeader.Height > bbi.lastHead {
		bbi.lastHead = header.RawHeader.Height
		bbi.blockTime.Record(
			ctx,
			time.Since(bbi.lastHeadTS).Milliseconds(),
			attribute.Int("height", int(header.RawHeader.Height)),
			attribute.String("state", "failed"),
		)
		bbi.lastHeadTS = time.Now()
	}

	return header, err
}

// IsSyncing returns the status of sync
func (bbi *blackBoxInstrument) IsSyncing(ctx context.Context) bool {
	return bbi.next.IsSyncing(ctx)
}
