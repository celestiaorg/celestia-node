package light

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/autobatch"
	"github.com/ipfs/go-datastore/namespace"
	ipldFormat "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/getters"
)

var (
	log                     = logging.Logger("share/light")
	cacheAvailabilityPrefix = datastore.NewKey("sampling_result")
	writeBatchSize          = 2048
)

// ShareAvailability implements share.Availability using Data Availability Sampling technique.
// It is light because it does not require the downloading of all the data to verify
// its availability. It is assumed that there are a lot of lightAvailability instances
// on the network doing sampling over the same Root to collectively verify its availability.
type ShareAvailability struct {
	getter share.Getter
	params Parameters

	// TODO(@Wondertan): Once we come to parallelized DASer, this lock becomes a contention point
	//  Related to #483
	// TODO: Striped locks? :D
	dsLk sync.RWMutex
	ds   *autobatch.Datastore
}

// NewShareAvailability creates a new light Availability.
func NewShareAvailability(
	getter share.Getter,
	ds datastore.Batching,
	opts ...Option,
) *ShareAvailability {
	params := DefaultParameters()
	ds = namespace.Wrap(ds, cacheAvailabilityPrefix)
	autoDS := autobatch.NewAutoBatching(ds, writeBatchSize)

	for _, opt := range opts {
		opt(&params)
	}

	return &ShareAvailability{
		getter: getter,
		params: params,
		ds:     autoDS,
	}
}

// SharesAvailable randomly samples `params.SampleAmount` amount of Shares committed to the given
// Root. This way SharesAvailable subjectively verifies that Shares are available.
func (la *ShareAvailability) SharesAvailable(ctx context.Context, dah *share.Root) error {
	// short-circuit if the given root is minimum DAH of an empty data square
	if share.DataHash(dah.Hash()).IsEmptyRoot() {
		return nil
	}

	// do not sample over Root that has already been sampled
	key := rootKey(dah)

	la.dsLk.RLock()
	exists, err := la.ds.Has(ctx, key)
	la.dsLk.RUnlock()
	if err != nil || exists {
		return err
	}

	log.Debugw("validate availability", "root", dah.String())
	// We assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	if err := dah.ValidateBasic(); err != nil {
		log.Errorw("availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		panic(err)
	}
	samples, err := SampleSquare(len(dah.RowRoots), int(la.params.SampleAmount))
	if err != nil {
		return err
	}

	// indicate to the share.Getter that a blockservice session should be created. This
	// functionality is optional and must be supported by the used share.Getter.
	ctx = getters.WithSession(ctx)

	log.Debugw("starting sampling session", "root", dah.String())
	errs := make(chan error, len(samples))
	for _, s := range samples {
		go func(s Sample) {
			log.Debugw("fetching share", "root", dah.String(), "row", s.Row, "col", s.Col)
			_, err := la.getter.GetShare(ctx, dah, s.Row, s.Col)
			if err != nil {
				log.Debugw("error fetching share", "root", dah.String(), "row", s.Row, "col", s.Col)
			}
			// we don't really care about Share bodies at this point
			// it also means we now saved the Share in local storage
			select {
			case errs <- err:
			case <-ctx.Done():
			}
		}(s)
	}

	for range samples {
		var err error
		select {
		case err = <-errs:
		case <-ctx.Done():
			err = ctx.Err()
		}

		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			log.Errorw("availability validation failed", "root", dah.String(), "err", err.Error())
			if ipldFormat.IsNotFound(err) || errors.Is(err, context.DeadlineExceeded) {
				return share.ErrNotAvailable
			}
			return err
		}
	}

	la.dsLk.Lock()
	err = la.ds.Put(ctx, key, []byte{})
	la.dsLk.Unlock()
	if err != nil {
		log.Errorw("storing root of successful SharesAvailable request to disk", "err", err)
	}
	return nil
}

// ProbabilityOfAvailability calculates the probability that the
// data square is available based on the amount of samples collected
// (params.SampleAmount).
//
// Formula: 1 - (0.75 ** amount of samples)
func (la *ShareAvailability) ProbabilityOfAvailability(context.Context) float64 {
	return 1 - math.Pow(0.75, float64(la.params.SampleAmount))
}

func rootKey(root *share.Root) datastore.Key {
	return datastore.NewKey(root.String())
}

// Close flushes all queued writes to disk.
func (la *ShareAvailability) Close(ctx context.Context) error {
	return la.ds.Flush(ctx)
}
