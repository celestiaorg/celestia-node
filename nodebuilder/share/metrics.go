package share

import (
	"context"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/service"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/utils/misc"
)

var (
	log   = logging.Logger("share-facade")
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

// shareServiceBlackboxInstru is the proxy struct
// used to perform measurements of blackbox metrics
// for the service.ShareService interface (i.e: the share service)
// check <insert documentation file here> for more info.
type shareServiceBlackboxInstru struct {
	// metrics
	requestsNum     syncint64.Counter
	requestDuration syncint64.Histogram
	requestSize     syncint64.Histogram

	// pointer to mod
	next service.ShareService
}

func newBlackBoxInstrument(next service.ShareService) (service.ShareService, error) {
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

	bbinstrument := &shareServiceBlackboxInstru{
		requestsNum,
		requestDuration,
		requestSize,
		next,
	}

	return bbinstrument, nil
}

func (bbi *shareServiceBlackboxInstru) Start(ctx context.Context) error {
	return bbi.next.Start(ctx)
}

func (bbi *shareServiceBlackboxInstru) Stop(ctx context.Context) error {
	return bbi.next.Stop(ctx)
}

func (bbi *shareServiceBlackboxInstru) GetShare(
	ctx context.Context,
	dah *share.Root,
	row, col int,
) (share.Share, error) {
	log.Debug("GetShare is being called through the facade")
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

func (bbi *shareServiceBlackboxInstru) GetShares(
	ctx context.Context,
	root *share.Root,
) ([][]share.Share, error) {
	return bbi.next.GetShares(ctx, root)
}

// GetSharesByNamespace iterates over a square's row roots and accumulates the found shares in the given namespace.ID.
func (bbi *shareServiceBlackboxInstru) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	namespace namespace.ID,
) ([]share.Share, error) {
	return bbi.next.GetSharesByNamespace(ctx, root, namespace)
}
