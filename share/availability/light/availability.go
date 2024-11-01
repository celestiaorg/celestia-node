package light

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

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
)

// ShareAvailability implements share.Availability using Data Availability Sampling technique.
// It is light because it does not require the downloading of all the data to verify
// its availability. It is assumed that there are a lot of lightAvailability instances
// on the network doing sampling over the same Root to collectively verify its availability.
type ShareAvailability struct {
	getter shwap.Getter
	params Parameters

	activeHeights *utils.Sessions
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
	ds = namespace.Wrap(ds, samplingResultsPrefix)
	autoDS := autobatch.NewAutoBatching(ds, writeBatchSize)

	for _, opt := range opts {
		opt(&params)
	}

	return &ShareAvailability{
		getter:        getter,
		params:        params,
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

	key := datastoreKeyForRoot(dah)
	samples := &SamplingResult{}

	// Attempt to load previous sampling results
	la.dsLk.RLock()
	data, err := la.ds.Get(ctx, key)
	la.dsLk.RUnlock()
	if err != nil {
		if !errors.Is(err, datastore.ErrNotFound) {
			return err
		}
		// No previous results; create new samples
		samples = NewSamplingResult(len(dah.RowRoots), int(la.params.SampleAmount))
	} else {
		err = json.Unmarshal(data, samples)
		if err != nil {
			return err
		}
		// Verify total samples count.
		totalSamples := len(samples.Remaining) + len(samples.Available)
		if totalSamples != int(la.params.SampleAmount) {
			return fmt.Errorf("invalid sampling result:"+
				" expected %d samples, got %d", la.params.SampleAmount, totalSamples)
		}
	}

	if len(samples.Remaining) == 0 {
		// All samples have been processed successfully
		return nil
	}

	var (
		mutex         sync.Mutex
		failedSamples []Sample
		wg            sync.WaitGroup
	)

	log.Debugw("starting sampling session", "height", header.Height())

	// remove one second from the deadline to ensure we have enough time to process the results
	samplingCtx, cancel := context.WithCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		samplingCtx, cancel = context.WithDeadline(ctx, deadline.Add(-time.Second))
	}
	defer cancel()

	// Concurrently sample shares
	for _, s := range samples.Remaining {
		wg.Add(1)
		go func(s Sample) {
			defer wg.Done()
			_, err := la.getter.GetShare(samplingCtx, header, s.Row, s.Col)
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				log.Debugw("error fetching share", "height", header.Height(), "row", s.Row, "col", s.Col)
				failedSamples = append(failedSamples, s)
			} else {
				samples.Available = append(samples.Available, s)
			}
		}(s)
	}
	wg.Wait()

	// Update remaining samples with failed ones
	samples.Remaining = failedSamples

	// Store the updated sampling result
	updatedData, err := json.Marshal(samples)
	if err != nil {
		return err
	}
	la.dsLk.Lock()
	err = la.ds.Put(ctx, key, updatedData)
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
