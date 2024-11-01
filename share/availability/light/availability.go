package light

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/autobatch"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var (
	log                   = logging.Logger("share/light")
	samplingResultsPrefix = datastore.NewKey("sampling_result")
	writeBatchSize        = 2048
	// sampleAmount is the amount of samples to take from the data square. It is defined as 16
	// to provide a good balance between the number of samples security guarantees it provides
	// for DAS of light clients.
	sampleAmount = 16
)

// ShareAvailability implements share.Availability using Data Availability Sampling technique.
// It is light because it does not require the downloading of all the data to verify
// its availability. It is assumed that there are a lot of lightAvailability instances
// on the network doing sampling over the same Root to collectively verify its availability.
type ShareAvailability struct {
	getter shwap.Getter

	activeHeights *utils.Sessions
	dsLk          sync.RWMutex
	ds            *autobatch.Datastore
}

// NewShareAvailability creates a new light Availability.
func NewShareAvailability(
	getter shwap.Getter,
	ds datastore.Batching,
) *ShareAvailability {
	ds = namespace.Wrap(ds, samplingResultsPrefix)
	autoDS := autobatch.NewAutoBatching(ds, writeBatchSize)

	return &ShareAvailability{
		getter:        getter,
		activeHeights: utils.NewSessions(),
		ds:            autoDS,
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
	release, err := la.activeHeights.StartSession(ctx, header.Height())
	if err != nil {
		return err
	}
	defer release()

	// load snapshot of the last sampling errors from disk
	key := datastoreKeyForRoot(dah)
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
		samples = selectRandomSamples(len(dah.RowRoots), sampleAmount)
	default:
		// Sampling result found, unmarshal it
		err = json.Unmarshal(last, &samples)
		if err != nil {
			return err
		}
	}

	var (
		failedSamplesLock sync.Mutex
		failedSamples     []Sample
	)

	log.Debugw("starting sampling session", "height", header.Height())
	var wg sync.WaitGroup
	for _, s := range samples {
		wg.Add(1)
		go func(s Sample) {
			defer wg.Done()
			// check if the sample is available
			_, err := la.getter.GetShare(ctx, header, s.Row, s.Col)
			if err != nil {
				log.Debugw("error fetching share", "height", header.Height(), "row", s.Row, "col", s.Col)
				failedSamplesLock.Lock()
				failedSamples = append(failedSamples, s)
				failedSamplesLock.Unlock()
			}
		}(s)
	}
	wg.Wait()

	// store the result of the sampling session
	bs, err := json.Marshal(failedSamples)
	if err != nil {
		return fmt.Errorf("failed to marshal sampling result: %w", err)
	}
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
		return share.ErrNotAvailable
	}
	return nil
}

func datastoreKeyForRoot(root *share.AxisRoots) datastore.Key {
	return datastore.NewKey(root.String())
}

// Close flushes all queued writes to disk.
func (la *ShareAvailability) Close(ctx context.Context) error {
	return la.ds.Flush(ctx)
}
