package light

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/autobatch"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
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
	bs     blockstore.Blockstore
	params Parameters

	storageWindow time.Duration

	activeHeights *utils.Sessions
	dsLk          sync.RWMutex
	ds            *autobatch.Datastore
}

// NewShareAvailability creates a new light Availability.
func NewShareAvailability(
	getter shwap.Getter,
	ds datastore.Batching,
	bs blockstore.Blockstore,
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
		bs:            bs,
		params:        params,
		storageWindow: availability.StorageWindow,
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

	// short-circuit if outside sampling window
	if !availability.IsWithinWindow(header.Time(), la.storageWindow) {
		return availability.ErrOutsideSamplingWindow
	}

	// Prevent multiple sampling and pruning sessions for the same header height
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
		if (totalSamples != int(la.params.SampleAmount)) && (totalSamples != len(dah.RowRoots)*len(dah.RowRoots)) {
			return fmt.Errorf("invalid sampling result:"+
				" expected %d samples, got %d", la.params.SampleAmount, totalSamples)
		}
	}

	if len(samples.Remaining) == 0 {
		// All samples have been processed successfully
		return nil
	}

	log.Debugw("starting sampling session", "root", dah.String())

	idxs := make([]shwap.SampleCoords, len(samples.Remaining))
	for i, s := range samples.Remaining {
		idxs[i] = shwap.SampleCoords{Row: s.Row, Col: s.Col}
	}

	// remove one second from the deadline to ensure we have enough time to process the results
	samplingCtx, cancel := context.WithCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		samplingCtx, cancel = context.WithDeadline(ctx, deadline.Add(-time.Second))
	}
	defer cancel()

	smpls, errGetSamples := la.getter.GetSamples(samplingCtx, header, idxs)
	if len(smpls) == 0 {
		return share.ErrNotAvailable
	}

	var failedSamples []shwap.SampleCoords

	for i, smpl := range smpls {
		if smpl.IsEmpty() {
			failedSamples = append(failedSamples, shwap.SampleCoords{Row: idxs[i].Row, Col: idxs[i].Col})
		} else {
			samples.Available = append(samples.Available, shwap.SampleCoords{Row: idxs[i].Row, Col: idxs[i].Col})
		}
	}

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
		return fmt.Errorf("store sampling result: %w", err)
	}

	if errors.Is(errGetSamples, context.Canceled) {
		// Availability did not complete due to context cancellation, return context error instead of
		// share.ErrNotAvailable
		return context.Canceled
	}

	// if any of the samples failed, return an error
	if len(failedSamples) > 0 {
		return share.ErrNotAvailable
	}

	return nil
}

// Prune deletes samples and all sampling data corresponding to provided header from store.
// The operation will remove all data that ShareAvailable might have created
func (la *ShareAvailability) Prune(ctx context.Context, h *header.ExtendedHeader) error {
	dah := h.DAH
	if share.DataHash(dah.Hash()).IsEmptyEDS() {
		return nil
	}

	// Prevent multiple sampling and pruning sessions for the same header height
	release, err := la.activeHeights.StartSession(ctx, h.Height())
	if err != nil {
		return err
	}
	defer release()

	key := datastoreKeyForRoot(dah)
	la.dsLk.RLock()
	data, err := la.ds.Get(ctx, key)
	la.dsLk.RUnlock()
	if errors.Is(err, datastore.ErrNotFound) {
		// nothing to prune
		return nil
	}
	if err != nil {
		return fmt.Errorf("get sampling result: %w", err)
	}

	var result SamplingResult
	err = json.Unmarshal(data, &result)
	if err != nil {
		return fmt.Errorf("unmarshal sampling result: %w", err)
	}

	// delete stored samples
	for _, sample := range result.Available {
		idx := shwap.SampleCoords{Row: sample.Row, Col: sample.Col}

		blk, err := bitswap.NewEmptySampleBlock(h.Height(), idx, len(h.DAH.RowRoots))
		if err != nil {
			return fmt.Errorf("marshal sample ID: %w", err)
		}
		err = la.bs.DeleteBlock(ctx, blk.CID())
		if err != nil {
			if !errors.Is(err, ipld.ErrNodeNotFound) {
				return fmt.Errorf("delete sample: %w", err)
			}
			log.Warnf("can't delete sample: %v, height: %v,  missing in blockstore", sample, h.Height())
		}
	}

	// delete the sampling result
	la.dsLk.Lock()
	err = la.ds.Delete(ctx, key)
	la.dsLk.Unlock()
	if err != nil {
		return fmt.Errorf("delete sampling result: %w", err)
	}
	return nil
}

func datastoreKeyForRoot(root *share.AxisRoots) datastore.Key {
	return datastore.NewKey(root.String())
}

// Close flushes all queued writes to disk.
func (la *ShareAvailability) Close(ctx context.Context) error {
	la.dsLk.Lock()
	defer la.dsLk.Unlock()
	return la.ds.Flush(ctx)
}
