package share

import (
	"context"
	"errors"

	format "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/rsmt2d"
)

// fullAvailability implements Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type fullAvailability struct {
	service format.DAGService
}

// NewFullAvailability creates a new full Availability.
func NewFullAvailability(service format.DAGService) Availability {
	return &fullAvailability{
		service: service,
	}
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *fullAvailability) SharesAvailable(ctx context.Context, root *Root) error {
	ctx, cancel := context.WithTimeout(ctx, AvailabilityTimeout)
	defer cancel()

	_, err := ipld.RetrieveData(ctx, root, fa.service, rsmt2d.NewRSGF8Codec())
	if err != nil {
		log.Errorw("availability validation failed", "root", root.Hash(), "err", err)
		if errors.Is(err, format.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) {
			return ErrNotAvailable
		}

		return err
	}
	return err
}
