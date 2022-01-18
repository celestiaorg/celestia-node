package share

import (
	"context"

	format "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/rsmt2d"
)

// fullAvailability implements Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type fullAvailability struct {
	getter format.NodeGetter
}

// NewFullAvailability creates a new full Availability.
func NewFullAvailability(get format.NodeGetter) Availability {
	return &fullAvailability{
		getter: get,
	}
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *fullAvailability) SharesAvailable(ctx context.Context, root *Root) error {
	_, err := ipld.RetrieveData(ctx, root, fa.getter, rsmt2d.NewRSGF8Codec())
	return err
}
