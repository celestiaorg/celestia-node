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

	if err := dah.ValidateBasic(); err != nil {
		log.Errorw("DAH validation failed", "error", err)
		return err
	}

	// indicate to the share.Getter that a blockservice session should be created. This
	// functionality is optional and must be supported by the used share.Getter.
	ctx = getters.WithSession(ctx)

	var (
		failedSamplesLock sync.Mutex
		failedSamples     []Sample
	)

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
				failedSamplesLock.Lock()
				failedSamples = append(failedSamples, s)
				failedSamplesLock.Unlock()
			}
		}(s)
	}
	wg.Wait()

	if errors.Is(ctx.Err(), context.Canceled) {
		// Availability did not complete due to context cancellation, return context error instead of
		// share.ErrNotAvailable
		return ctx.Err()
	}

	// store the result of the sampling session
	bs := encodeSamples(failedSamples)
	la.dsLk.Lock()
	err = la.ds.Put(ctx, key, bs)
	la.dsLk.Unlock()
	if err != nil {
		log.Errorw("Failed to store sampling result", "error", err)
	}

	// if any of the samples failed, return an error
	if len(failedSamples) > 0 {
		log.Errorw("availability validation failed",
			"root", dah.String(),
			"failed_samples", failedSamples,
		)
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
