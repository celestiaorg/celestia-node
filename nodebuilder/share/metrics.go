package share

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/rsmt2d"

	// "github.com/celestiaorg/celestia-node/share/service"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/utils/misc"
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

	squareSizeMetricName = "node.share.blackbox.eds_size"
	squareSizeMetricDesc = "size of the erasure coded block"
)

// instrumentedShareGetter is the proxy struct
// used to perform "blackbox" metrics measurements
// for the share.Getter interface implementers
// check share/getter.go and share/availability/light.go for more info
type instrumentedShareGetter struct {
	// metrics
	requestsNum     syncint64.Counter
	requestDuration syncint64.Histogram
	requestSize     syncint64.Histogram
	squareSize      syncint64.Histogram

	// pointer to mod
	next share.Getter
}

func newInstrument(next share.Getter) (share.Getter, error) {
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

	squareSize, err := meter.
		SyncInt64().
		Histogram(
			squareSizeMetricName,
			instrument.WithDescription(squareSizeMetricDesc),
		)
	if err != nil {
		return nil, err
	}

	instrument := &instrumentedShareGetter{
		requestsNum,
		requestDuration,
		requestSize,
		squareSize,
		next,
	}

	return instrument, nil
}

// // GetShare gets a Share by coordinates in EDS.
func (ins *instrumentedShareGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	log.Debug("GetShare is being called through the facade")
	now := time.Now()
	requestID, err := misc.RandString(5)
	if err != nil {
		return nil, err
	}

	// defer recording the duration until the request has received a response and finished
	defer func(ctx context.Context, begin time.Time) {
		ins.requestDuration.Record(
			ctx,
			time.Since(begin).Milliseconds(),
		)
	}(ctx, now)

	// measure the EDS size
	// this will track the EDS size for light nodes
	ins.squareSize.Record(
		ctx,
		int64(len(root.RowsRoots)),
		attribute.String("request-id", requestID),
	)

	// perform the actual request
	share, err := ins.next.GetShare(ctx, root, row, col)
	if err != nil {
		// count the request and tag it as a failed one
		ins.requestsNum.Add(
			ctx,
			1,
			attribute.String("request-id", requestID),
			attribute.String("state", "failed"),
		)
		return share, err
	}

	// other wise, count the request but tag it as a succeeded one
	ins.requestsNum.Add(
		ctx,
		1,
		attribute.String("request-id", requestID),
		attribute.String("state", "succeeded"),
	)

	// record the response size (extended header in this case)
	ins.requestSize.Record(
		ctx,
		int64(len(share)),
	)

	return share, err
}

// // GetEDS gets the full EDS identified by the given root.
func (ins *instrumentedShareGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	// measure the EDS size
	// this will track the EDS size for full nodes
	requestID, err := misc.RandString(5)
	if err != nil {
		return nil, err
	}

	ins.squareSize.Record(
		ctx,
		int64(len(root.RowsRoots)),
		attribute.String("request-id", requestID),
	)

	return ins.next.GetEDS(ctx, root)
}

// // GetSharesByNamespace gets all shares from an EDS within the given namespace.
// // Shares are returned in a row-by-row order if the namespace spans multiple rows.
func (ins *instrumentedShareGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	id namespace.ID,
) (share.NamespacedShares, error) {
	return ins.next.GetSharesByNamespace(ctx, root, id)
}
