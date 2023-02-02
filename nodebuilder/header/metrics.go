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
	meter = global.MeterProvider().Meter("proxy-header")
)

// blackBoxInstrument is the proxy struct
// used to perform measurements of blackbox metrics
// for the header.Module interface (i.e: the header service)
// check <insert documentation file here> for more info.
// TODO(@derrandz): create doc file and link to it in here
type blackBoxInstrument struct {
	// metrics
	requestCount    syncint64.Counter
	requestDuration syncint64.Histogram
	responseSize    syncint64.Histogram
	blockTime       syncint64.Histogram

	lastHeadTimestamp time.Time
	lastHead          int64

	// pointer to mod
	next Module
}

// constructor that returns a proxied
// interface for blackbox metrics
func newBlackBoxInstrument(next Module) (Module, error) {
	requestsCount, err := meter.
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

	responseSize, err := meter.
		SyncInt64().
		Histogram(
			"node.header.blackbox.response_size",
			instrument.WithDescription("size of a get header response"),
		)
	if err != nil {
		return nil, err
	}

	blockTime, err := meter.
		SyncInt64().
		Histogram(
			"node.header.blackbox.block_time",
			instrument.WithDescription("block time"),
		)
	if err != nil {
		return nil, err
	}

	lastHeadTS := time.Now()
	lastHead := int64(0)

	return &blackBoxInstrument{
		requestsCount,
		requestDuration,
		responseSize,
		blockTime,
		lastHeadTS,
		lastHead,
		next,
	}, nil
}

// GetByHeight returns the ExtendedHeader at the given height, blocking
// until header has been processed by the store or context deadline is exceeded.
func (bbi *blackBoxInstrument) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	start := time.Now()
	requestID, err := misc.RandString(5)
	if err != nil {
		return nil, err
	}

	// defer recording the duration until the request has received a response and finished
	defer func() {
		bbi.requestDuration.Record(
			ctx,
			time.Since(start).Milliseconds(),
		)
	}()

	eh, err := bbi.next.GetByHeight(ctx, height)
	if err != nil {
		bbi.requestCount.Add(
			ctx,
			1,
			attribute.String("request-id", requestID),
			attribute.String("state", "failed"),
		)
		return eh, err
	}

	bbi.requestCount.Add(
		ctx,
		1,
		attribute.String("request-id", requestID),
		attribute.String("state", "succeeded"),
	)

	// retrieve the binary format to get the size of the header
	// TODO(@derrandz): remove this in favor of a metric that records
	// the size of the actual network response (and not the header size)
	bin, err := eh.MarshalBinary()
	if err != nil {
		return nil, err
	}

	bbi.responseSize.Record(
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
			time.Since(bbi.lastHeadTimestamp).Milliseconds(),
			attribute.Int("height", int(header.RawHeader.Height)),
		)
		bbi.lastHeadTimestamp = time.Now()
	}

	return header, err
}

// IsSyncing returns the status of sync
func (bbi *blackBoxInstrument) IsSyncing(ctx context.Context) bool {
	return bbi.next.IsSyncing(ctx)
}
