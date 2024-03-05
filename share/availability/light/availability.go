package light

import (
	"context"
	"errors"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/autobatch"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
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
// ExtendedHeader. This way SharesAvailable subjectively verifies that Shares are available.
func (la *ShareAvailability) SharesAvailable(ctx context.Context, header *header.ExtendedHeader) error {
	dah := header.DAH
	// short-circuit if the given root is minimum DAH of an empty data square
	if share.DataHash(dah.Hash()).IsEmptyRoot() {
		return nil
	}

	// load snapshot of the last sampling errors from disk
	key := rootKey(dah)
	la.dsLk.RLock()
	last, err := la.ds.Get(ctx, key)
	la.dsLk.RUnlock()

	// Check for error cases
	var samples []Sample
	switch {
	case err == nil && len(last) == 0:
		// Availability has already been validated
		return nil
	case err != nil && !errors.Is(err, datastore.ErrNotFound):
		// Other error occurred
		return err
	case errors.Is(err, datastore.ErrNotFound):
		// No sampling result found, select new samples
		samples, err = SampleSquare(len(dah.RowRoots), int(la.params.SampleAmount))
		if err != nil {
			return err
		}
	default:
		// Sampling result found, unmarshal it
		samples, err = decodeSamples(last)
		if err != nil {
			return err
		}
	}

	// We assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	if err := dah.ValidateBasic(); err != nil {
		log.Errorw("availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		return err
	}

	// indicate to the share.Getter that a blockservice session should be created. This
	// functionality is optional and must be supported by the used share.Getter.
	ctx = getters.WithSession(ctx)

	result := struct {
		sync.Mutex
		failed []Sample
	}{}

	log.Debugw("starting sampling session", "root", dah.String())
	var wg sync.WaitGroup
	for _, s := range samples {
		wg.Add(1)
		go func(s Sample) {
			defer wg.Done()
			// check if the sample is available
			_, err := la.getter.GetShare(ctx, header, int(s.Row), int(s.Col))
			if err != nil {
				log.Debugw("error fetching share", "root", dah.String(), "row", s.Row, "col", s.Col)
				result.Lock()
				result.failed = append(result.failed, s)
				result.Unlock()
			}
		}(s)
	}
	wg.Wait()

	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}

	// store the result of the sampling session
	bs := encodeSamples(result.failed)
	la.dsLk.Lock()
	err = la.ds.Put(ctx, key, bs)
	la.dsLk.Unlock()
	if err != nil {
		log.Errorw("storing root of successful SharesAvailable request to disk", "err", err)
	}

	// if any of the samples failed, return an error
	if len(result.failed) > 0 {
		log.Errorw("availability validation failed", "root", dah.String(), "failed_samples", result.failed)
		return share.ErrNotAvailable
	}
	return nil
}

func rootKey(root *share.Root) datastore.Key {
	return datastore.NewKey(root.String())
}

// Close flushes all queued writes to disk.
func (la *ShareAvailability) Close(ctx context.Context) error {
	return la.ds.Flush(ctx)
}
