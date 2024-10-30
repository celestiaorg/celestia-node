package light

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/autobatch"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
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
	getter shwap.Getter
	params Parameters

	activeHeights sync.Map // Tracks active sampling sessions by height
	dsLk          sync.RWMutex
	ds            *autobatch.Datastore
}

// NewShareAvailability creates a new light Availability.
func NewShareAvailability(
	getter shwap.Getter,
	ds datastore.Batching,
	opts ...Option,
) *ShareAvailability {
	params := *DefaultParameters()
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
	// short-circuit if the given root is an empty data square
	if share.DataHash(dah.Hash()).IsEmptyEDS() {
		return nil
	}

	// Prevent multiple sampling sessions for the same header height
	release, err := la.startSamplingSession(ctx, header)
	if err != nil {
		return err
	}
	defer release()

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

	// store the result of the sampling session
	bs := encodeSamples(failedSamples)
	la.dsLk.Lock()
	err = la.ds.Put(ctx, key, bs)
	la.dsLk.Unlock()
	if err != nil {
		return fmt.Errorf("failed to store sampling result: %w", err)
	}

	if errors.Is(ctx.Err(), context.Canceled) {
		// Availability did not complete due to context cancellation, return context error instead of
		// share.ErrNotAvailable
		return ctx.Err()
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

// startSamplingSession manages concurrent sampling sessions for the specified header height.
// It ensures only one sampling session can proceed for each height, avoiding duplicate efforts.
// If a session is already active for the given height, it waits until the session completes or
// context error occurs. It provides a release function to clean up the session lock for this
// height, once the sampling session is complete.
func (la *ShareAvailability) startSamplingSession(
	ctx context.Context,
	header *header.ExtendedHeader,
) (releaseLock func(), err error) {
	// Attempt to load or initialize a channel to track the sampling session for this height
	lockChan, alreadyActive := la.activeHeights.LoadOrStore(header.Height(), make(chan struct{}))
	if alreadyActive {
		// If a session is already active, wait for it to complete
		select {
		case <-lockChan.(chan struct{}):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return func() {}, nil
	}

	// Provide a function to release the lock once sampling is complete
	releaseLock = func() {
		close(lockChan.(chan struct{}))
		la.activeHeights.Delete(header.Height())
	}
	return releaseLock, nil
}

func rootKey(root *share.AxisRoots) datastore.Key {
	return datastore.NewKey(root.String())
}

// Close flushes all queued writes to disk.
func (la *ShareAvailability) Close(ctx context.Context) error {
	return la.ds.Flush(ctx)
}
