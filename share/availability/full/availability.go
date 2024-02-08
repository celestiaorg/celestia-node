package full

import (
	"context"
	"errors"
	"fmt"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/rsmt2d"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/store"
)

var log = logging.Logger("share/full")

// ShareAvailability implements share.Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type ShareAvailability struct {
	store  *store.Store
	getter share.Getter
	disc   *discovery.Discovery

	cancel context.CancelFunc
}

// NewShareAvailability creates a new full ShareAvailability.
func NewShareAvailability(
	store *store.Store,
	getter share.Getter,
	disc *discovery.Discovery,
) *ShareAvailability {
	return &ShareAvailability{
		store:  store,
		getter: getter,
		disc:   disc,
	}
}

func (fa *ShareAvailability) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	fa.cancel = cancel

	go fa.disc.Advertise(ctx)
	return nil
}

func (fa *ShareAvailability) Stop(context.Context) error {
	fa.cancel()
	return nil
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *ShareAvailability) SharesAvailable(ctx context.Context, header *header.ExtendedHeader) error {
	// a hack to avoid loading the whole EDS in mem if we store it already.
	if ok, _ := fa.store.HasByHeight(ctx, header.Height()); ok {
		return nil
	}

	eds, err := fa.getEds(ctx, header)
	if err != nil {
		return err
	}

	f, err := fa.store.Put(ctx, header.DAH.Hash(), header.Height(), eds)
	if err != nil {
		return fmt.Errorf("full availability: failed to store eds: %w", err)
	}
	utils.CloseAndLog(log, "file", f)
	return nil
}

func (fa *ShareAvailability) getEds(ctx context.Context, header *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	dah := header.DAH
	// short-circuit if the given root is minimum DAH of an empty data square, to avoid datastore hit
	if share.DataHash(dah.Hash()).IsEmptyRoot() {
		return share.EmptyExtendedDataSquare(), nil
	}

	eds, err := fa.getter.GetEDS(ctx, header)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		log.Errorw("availability validation failed", "root", dah.String(), "err", err.Error())
		var byzantineErr *byzantine.ErrByzantine
		if errors.Is(err, share.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) && !errors.As(err, &byzantineErr) {
			return nil, share.ErrNotAvailable
		}
		return nil, err
	}
	return eds, nil
}
