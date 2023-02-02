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

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/utils/misc"
)

var (
	meter = global.MeterProvider().Meter("proxy-share")
)

// instrumentedShareGetter is the proxy struct
// used to perform "blackbox" metrics measurements
// for the share.Getter interface implementers
// check share/getter.go and share/availability/light.go for more info
type instrumentedShareGetter struct {
	// metrics
	requestCount    syncint64.Counter
	requestDuration syncint64.Histogram
	requestSize     syncint64.Histogram
	squareSize      syncint64.Histogram

	// pointer to mod
	next share.Getter
}

func newInstrument(next share.Getter) (share.Getter, error) {
	requestCount, err := meter.
		SyncInt64().
		Counter(
			"node.share.blackbox.requests_count",
			instrument.WithDescription("get share requests count"),
		)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.
		SyncInt64().
		Histogram(
			"node.share.blackbox.request_duration",
			instrument.WithDescription("duration of a single get share request"),
		)
	if err != nil {
		return nil, err
	}

	requestSize, err := meter.
		SyncInt64().
		Histogram(
			"node.share.blackbox.request_size",
			instrument.WithDescription("size of a get share response"),
		)
	if err != nil {
		return nil, err
	}

	squareSize, err := meter.
		SyncInt64().
		Histogram(
			"node.share.blackbox.eds_size",
			instrument.WithDescription("size of the erasure coded block"),
		)
	if err != nil {
		return nil, err
	}

	return &instrumentedShareGetter{
		requestCount,
		requestDuration,
		requestSize,
		squareSize,
		next,
	}, nil
}

// GetShare gets a Share by coordinates in EDS.
func (ins *instrumentedShareGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	log.Debug("proxy-share: GetShare call is being proxied")
	start := time.Now()
	requestID, err := misc.RandString(5)
	if err != nil {
		return nil, err
	}

	// defer recording the duration until the request has received a response and finished
	defer func() {
		ins.requestDuration.Record(
			ctx,
			time.Since(start).Milliseconds(),
		)
	}()

	// measure the EDS size (or rather the `k` parameter)
	// this will track the EDS size for light nodes
	ins.squareSize.Record(
		ctx,
		int64(len(root.RowsRoots)),
		attribute.String("request-id", requestID),
	)

	share, err := ins.next.GetShare(ctx, root, row, col)
	if err != nil {
		ins.requestCount.Add(
			ctx,
			1,
			attribute.String("request-id", requestID),
			attribute.String("state", "failed"),
		)
		return share, err
	}

	ins.requestCount.Add(
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
