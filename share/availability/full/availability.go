package full

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/dagstore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var log = logging.Logger("share/full")

// ShareAvailability implements share.Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type ShareAvailability struct {
	store  *eds.Store
	getter share.Getter
}

// NewShareAvailability creates a new full ShareAvailability.
func NewShareAvailability(
	store *eds.Store,
	getter share.Getter,
) *ShareAvailability {
	return &ShareAvailability{
		store:  store,
		getter: getter,
	}
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *ShareAvailability) SharesAvailable(ctx context.Context, header *header.ExtendedHeader) error {
	dah := header.DAH
	// short-circuit if the given root is minimum DAH of an empty data square, to avoid datastore hit
	if share.DataHash(dah.Hash()).IsEmptyRoot() {
		return nil
	}

	// we assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	if err := dah.ValidateBasic(); err != nil {
		log.Errorw("Availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		panic(err)
	}

	// a hack to avoid loading the whole EDS in mem if we store it already.
	if ok, _ := fa.store.Has(ctx, dah.Hash()); ok {
		return nil
	}

	adder := ipld.NewProofsAdder(len(dah.RowRoots))
	ctx = ipld.CtxWithProofsAdder(ctx, adder)
	defer adder.Purge()

	eds, err := fa.getter.GetEDS(ctx, header)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		log.Errorw("availability validation failed", "root", dah.String(), "err", err.Error())
		var byzantineErr *byzantine.ErrByzantine
		if errors.Is(err, share.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) && !errors.As(err, &byzantineErr) {
			return share.ErrNotAvailable
		}
		return err
	}

	err = fa.store.Put(ctx, dah.Hash(), eds)
	if err != nil && !errors.Is(err, dagstore.ErrShardExists) {
		return fmt.Errorf("full availability: failed to store eds: %w", err)
	}
	return nil
}
