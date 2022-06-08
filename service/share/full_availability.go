package share

import (
	"context"
	"errors"
	"time"

	"github.com/ipfs/go-blockservice"
	format "github.com/ipfs/go-ipld-format"
	"go.opentelemetry.io/otel"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/telemetry"
)

const tracerName = "fullAvailability"

// fullAvailability implements Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type fullAvailability struct {
	rtrv *ipld.Retriever
}

// NewFullAvailability creates a new full Availability.
func NewFullAvailability(bServ blockservice.BlockService) Availability {
	return &fullAvailability{
		rtrv: ipld.NewRetriever(bServ),
	}
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *fullAvailability) SharesAvailable(ctx context.Context, root *Root) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "SharesAvailable")
	defer span.End()

	startTime := time.Now()
	defer func() {
		telemetry.RecordSharesAvailable(ctx, time.Since(startTime))
	}()

	telemetry.IncSharesAvailable(ctx)

	ctx, cancel := context.WithTimeout(ctx, AvailabilityTimeout)
	defer cancel()

	_, err := fa.rtrv.Retrieve(ctx, root)
	if err != nil {
		log.Errorw("availability validation failed", "root", root.Hash(), "err", err)
		if errors.Is(err, format.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) {
			return ErrNotAvailable
		}

		return err
	}
	return err
}
