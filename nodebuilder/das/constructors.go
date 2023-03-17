package das

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fraud"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

var _ Module = (*daserStub)(nil)

var errStub = fmt.Errorf("module/das: stubbed: dasing is not available on bridge nodes")

// daserStub is a stub implementation of the DASer that is used on bridge nodes, so that we can
// provide a friendlier error when users try to access the daser over the API.
type daserStub struct{}

func (d daserStub) SamplingStats(context.Context) (das.SamplingStats, error) {
	return das.SamplingStats{}, errStub
}

func (d daserStub) WaitCatchUp(context.Context) error {
	return errStub
}

func newDaserStub() Module {
	return &daserStub{}
}

func newDASer(
	da share.Availability,
	hsub libhead.Subscriber[*header.ExtendedHeader],
	store libhead.Store[*header.ExtendedHeader],
	batching datastore.Batching,
	fraudServ fraud.Service,
	bFn shrexsub.BroadcastFn,
	options ...das.Option,
) (*das.DASer, error) {
	return das.NewDASer(da, hsub, store, batching, fraudServ, bFn, options...)
}
