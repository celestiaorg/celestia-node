package das

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

var _ Module = (*daserStub)(nil)

// daserStub is a stub implementation of the DASer that is used on bridge nodes, so that we can
// provide a friendlier error when users try to access the daser over the API.
type daserStub struct{}

func (d daserStub) SamplingStats(context.Context) (das.SamplingStats, error) {
	return das.SamplingStats{}, fmt.Errorf("moddas: dasing is not available on bridge nodes")
}

func newDaserStub() Module {
	return &daserStub{}
}

func NewDASer(
	da share.Availability,
	hsub header.Subscriber,
	store header.Store,
	batching datastore.Batching,
	fraudService fraud.Service,
	options ...das.Option,
) (*das.DASer, error) {
	return das.NewDASer(da, hsub, store, batching, fraudService, options...)
}
