package share

import (
	"context"
	"math/rand"
	"time"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

var (
	meter = global.MeterProvider().Meter("blackbox-share")
)

const (
	// requestsNum metric attributes
	requestsNumMetricName = "node.share.blackbox.requests_count"
	requestNumMetricDesc  = "get share requests count"

	// reqeustDuration metric attributes
	requestDurationMetricName = "node.share.blackbox.request_duration"
	requestDurationMetricDesc = "duration of a single get share request"

	// requestSize metric attributes
	requestSizeMetricName = "node.share.blackbox.request_size"
	requestSizeMetricDesc = "size of a get share response"
)

// blackBoxInstrument is the proxy struct
// used to perform measurements of blackbox metrics
// for the share.Module interface (i.e: the share service)
// check <insert documentation file here> for more info.
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
			requestsNumMetricName,
			instrument.WithDescription(requestNumMetricDesc),
		)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.
		SyncInt64().
		Histogram(
			requestDurationMetricName,
			instrument.WithDescription(requestDurationMetricDesc),
		)
	if err != nil {
		return nil, err
	}

	requestSize, err := meter.
		SyncInt64().
		Histogram(
			requestSizeMetricName,
			instrument.WithDescription(requestSizeMetricDesc),
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

// SharesAvailable subjectively validates if Shares committed to the given Root are available on the Network.
func (bbi *blackBoxInstrument) SharesAvailable(ctx context.Context, root *share.Root) error {
	return bbi.next.SharesAvailable(ctx, root)
}

// ProbabilityOfAvailability calculates the probability of the data square
// being available based on the number of samples collected.
func (bbi *blackBoxInstrument) ProbabilityOfAvailability() float64 {
	return bbi.next.ProbabilityOfAvailability()
}

func (bbi *blackBoxInstrument) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	now := time.Now()
	requestID := RandStringBytes(5)

	// defer recording the duration until the request has received a response and finished
	defer func(ctx context.Context, begin time.Time) {
		bbi.requestDuration.Record(
			ctx,
			time.Since(begin).Milliseconds(),
		)
	}(ctx, now)

	// perform the actual request
	share, err := bbi.next.GetShare(ctx, dah, row, col)
	if err != nil {
		// count the request and tag it as a failed one
		bbi.requestsNum.Add(
			ctx,
			1,
			attribute.String("request-id", requestID),
			attribute.String("state", "failed"),
		)
		return share, err
	}

	// other wise, count the request but tag it as a succeeded one
	bbi.requestsNum.Add(
		ctx,
		1,
		attribute.String("request-id", requestID),
		attribute.String("state", "succeeded"),
	)

	// record the response size (extended header in this case)
	bbi.requestSize.Record(
		ctx,
		int64(len(share)),
	)

	return share, err
}

func (bbi *blackBoxInstrument) GetShares(ctx context.Context, root *share.Root) ([][]share.Share, error) {
	return bbi.next.GetShares(ctx, root)
}

// GetSharesByNamespace iterates over a square's row roots and accumulates the found shares in the given namespace.ID.
func (bbi *blackBoxInstrument) GetSharesByNamespace(ctx context.Context, root *share.Root, namespace namespace.ID) ([]share.Share, error) {
	return bbi.next.GetSharesByNamespace(ctx, root, namespace)
}

// this is currently duplicated (look in nodebuilder/header/metric.go) until I find a good place to factor it in
// utility: copy-pasta from the internet to get this working
// TODO(@derrandz): find a better way for generating random unique IDs for requests
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
