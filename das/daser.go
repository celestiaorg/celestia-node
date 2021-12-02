package das

import (
	"context"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

var log = logging.Logger("das")

// DASer continuously validates availability of data committed to headers.
// TODO(@Wondertan): Start and Stop is better be thread-safe.
type DASer struct {
	da   share.Availability
	hsub header.Subscriber

	cancel context.CancelFunc
	done   chan struct{}
}

// NewDASer creates a new DASer.
func NewDASer(da share.Availability, hsub header.Subscriber) *DASer {
	return &DASer{
		da:   da,
		hsub: hsub,
		done: make(chan struct{}),
	}
}

// Start initiates subscription for new ExtendedHeaders and spawns a sampling routine.
func (d *DASer) Start(context.Context) error {
	if d.cancel != nil {
		return fmt.Errorf("da: DASer already started")
	}

	sub, err := d.hsub.Subscribe()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	go d.sampling(ctx, sub)
	d.cancel = cancel
	return nil
}

// Stop stops sampling.
func (d *DASer) Stop(ctx context.Context) error {
	if d.cancel == nil {
		return fmt.Errorf("da: DASer already stopped")
	}

	d.cancel()
	select {
	case <-d.done:
		d.cancel = nil
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sampling validates availability for each Header received from header subscription.
func (d *DASer) sampling(ctx context.Context, sub header.Subscription) {
	defer sub.Cancel()
	defer close(d.done)
	for {
		h, err := sub.NextHeader(ctx)
		if err != nil {
			if err == context.Canceled {
				return
			}

			log.Errorw("DASer failed to get next header", "err", err)
			continue
		}

		startTime := time.Now()

		err = d.da.SharesAvailable(ctx, h.DAH)
		if err != nil {
			if err == context.Canceled {
				return
			}
			log.Errorw("validation failed", "root", h.DAH.Hash(), "err", err)
			// continue sampling
		}

		sampleTime := time.Since(startTime)
		log.Infow("sampling successful", "height", h.Height, "hash", h.Hash(), "square width",
			len(h.DAH.RowsRoots), "finished (s)", sampleTime.Seconds())
	}
}
