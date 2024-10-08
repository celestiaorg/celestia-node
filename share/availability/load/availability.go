package laod_test

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

var (
	log   = logging.Logger("share/light")
	meter = otel.Meter("load_test")
)

// ShareAvailability implements share.Availability using Data Availability Sampling technique.
// It is light because it does not require the downloading of all the data to verify
// its availability. It is assumed that there are a lot of lightAvailability instances
// on the network doing sampling over the same Root to collectively verify its availability.
type ShareAvailability struct {
	getter share.Getter

	sample metric.Float64Histogram
}

// NewShareAvailability creates a new light Availability.
func NewShareAvailability(
	getter share.Getter,
) *ShareAvailability {
	return &ShareAvailability{
		getter: getter,
	}
}

// SharesAvailable randomly samples `params.SampleAmount` amount of Shares committed to the given
// ExtendedHeader. This way SharesAvailable subjectively verifies that Shares are available.
func (la *ShareAvailability) SharesAvailable(ctx context.Context, header *header.ExtendedHeader) error {
	now := time.Now()
	size := len(header.DAH.RowRoots)
	row, col := rand.Intn(size), rand.Intn(size)
	fmt.Println("TRY AVAILABILITY")
	_, err := la.getter.GetShare(ctx, header, row, col)
	if la.sample != nil {
		la.sample.Record(ctx, time.Since(now).Seconds(),
			metric.WithAttributes(attribute.Int("header_width", size)),
			metric.WithAttributes(attribute.Bool("failed", err != nil)),
		)
	}
	if err != nil {
		log.Errorf("LOADTEST: failed to sample share height=%d row=%d col=%d err=%s after: %v",
			header.Height(), header.Hash(), row, col, err, time.Since(now))
		return err
	}
	return nil
}

func (la *ShareAvailability) WithMetrics() error {
	sample, err := meter.Float64Histogram("load_test_sample_time_hist",
		metric.WithDescription("duration of sampling a single header"))
	if err != nil {
		return err
	}
	la.sample = sample
	return nil
}
