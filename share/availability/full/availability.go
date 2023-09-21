package full

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/dagstore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/p2p/discovery"
)

var log = logging.Logger("share/full")

// ShareAvailability implements share.Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type ShareAvailability struct {
	store  *eds.Store
	getter share.Getter
	disc   *discovery.Discovery

	cancel context.CancelFunc
}

// NewShareAvailability creates a new full ShareAvailability.
func NewShareAvailability(
	store *eds.Store,
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
func (fa *ShareAvailability) SharesAvailable(ctx context.Context, root *share.Root) error {
	// short-circuit if the given root is minimum DAH of an empty data square, to avoid datastore hit
	if share.DataHash(root.Hash()).IsEmptyRoot() {
		return nil
	}

	// we assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	if err := root.ValidateBasic(); err != nil {
		log.Errorw("Availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		panic(err)
	}

	// a hack to avoid loading the whole EDS in mem if we store it already.
	if ok, _ := fa.store.Has(ctx, root.Hash()); ok {
		return nil
	}

	adder := ipld.NewProofsAdder(len(root.RowRoots))
	ctx = ipld.CtxWithProofsAdder(ctx, adder)
	defer adder.Purge()

	eds, err := fa.getter.GetEDS(ctx, root)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		log.Errorw("availability validation failed", "root", root.String(), "err", err.Error())
		var byzantineErr *byzantine.ErrByzantine
		if errors.Is(err, share.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) && !errors.As(err, &byzantineErr) {
			return share.ErrNotAvailable
		}
		return err
	}

	err = fa.store.Put(ctx, root.Hash(), eds)
	if err != nil && !errors.Is(err, dagstore.ErrShardExists) {
		return fmt.Errorf("full availability: failed to store eds: %w", err)
	}
	return nil
}

func (fa *ShareAvailability) ProbabilityOfAvailability(context.Context) float64 {
	return 1
}
