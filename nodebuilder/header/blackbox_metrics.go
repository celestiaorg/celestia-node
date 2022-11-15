package header

import (
	"context"
	"math/rand"
	"time"

	"github.com/celestiaorg/celestia-node/header"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

var (
	meter = global.MeterProvider().Meter("blackbox-header")
)

type blackBoxInstrument struct {
	// metrics
	requestsNum     syncint64.Counter
	requestDuration syncint64.Histogram
	requestSize     syncint64.Histogram

	// pointer to mod
	next Module
}

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

	bbinstrument := &blackBoxInstrument{
		requestsNum,
		requestDuration,
		requestSize,
		next,
	}

	return bbinstrument, nil
}

// GetByHeight returns the ExtendedHeader at the given height, blocking
// until header has been processed by the store or context deadline is exceeded.
func (bbi *blackBoxInstrument) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	now := time.Now()
	requestId := RandStringBytes(5)

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
			attribute.String("request-id", requestId),
			attribute.String("state", "failed"),
		)
		return eh, err
	}

	// other wise, count the request but tag it as a succeeded one
	bbi.requestsNum.Add(
		ctx,
		1,
		attribute.String("request-id", requestId),
		attribute.String("state", "succeeded"),
	)

	// retrieve the binary format to get the size of the header
	// TODO(@team): is ExtendedHeader.MarshalBinary() == ResponseSize? I am making this assumption for now
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

// IsSyncing returns the status of sync
func (bbi *blackBoxInstrument) IsSyncing() bool {
	return bbi.next.IsSyncing()
}

// utility
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
