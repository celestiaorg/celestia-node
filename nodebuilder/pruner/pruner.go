package pruner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

const (
	dsPrefix = "/pruner/epoch/"
)

var log = logging.Logger("pruner")

type StoragePruner struct {
	cancel context.CancelFunc
	cfg    Config

	oldestEpoch atomic.Uint64

	stripedLocks [256]sync.Mutex
	activeEpochs map[uint64]struct{}

	ds    datastore.Batching
	store *eds.Store

	done chan struct{}
}

func NewStoragePruner(ds datastore.Batching, store *eds.Store, cfg Config) *StoragePruner {
	sp := &StoragePruner{
		activeEpochs: make(map[uint64]struct{}),
		ds:           namespace.Wrap(ds, datastore.NewKey(dsPrefix)),
		store:        store,
		cfg:          cfg,
		done:         make(chan struct{}),
	}

	// set to max uint64 as sentinel before state is restored or first epoch is registered
	sp.oldestEpoch.Store(^uint64(0))
	return sp
}

func (sp *StoragePruner) Start(ctx context.Context) error {
	err := sp.restoreState(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	sp.cancel = cancel

	go sp.run(ctx)
	return nil
}

func (sp *StoragePruner) Stop(_ context.Context) error {
	sp.cancel()
	<-sp.done
	return nil
}

func (sp *StoragePruner) restoreState(ctx context.Context) error {
	results, err := sp.ds.Query(ctx, query.Query{})
	if err != nil {
		return fmt.Errorf("failed to recover pruner state from datastore: %w", err)
	}
	for {
		res, ok := results.NextSync()
		if !ok {
			break
		}
		epoch, err := strconv.ParseUint(res.Key[1:], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse epoch from datastore: %w", err)
		}
		// we don't need to use locks here because no methods will be called until the callers have also started
		sp.activeEpochs[epoch] = struct{}{}
		oldest := sp.oldestEpoch.Load()
		if epoch < oldest {
			sp.oldestEpoch.CompareAndSwap(oldest, epoch)
		}
	}
	log.Infow("restored state from datastore",
		"oldestEpoch", sp.oldestEpoch.Load(),
		"active epoch count", len(sp.activeEpochs),
	)
	return nil
}

func (sp *StoragePruner) Register(ctx context.Context, h *header.ExtendedHeader) error {
	var datahashes []share.DataHash
	var err error

	if share.DataHash(h.DAH.Hash()).IsEmptyRoot() {
		return nil
	}

	epoch := sp.calculateEpoch(h.Time())
	lk := &sp.stripedLocks[epoch%256]
	lk.Lock()
	defer lk.Unlock()
	log.Debugf("registering datahash %X to epoch %d", h.DAH.Hash(), epoch)
	_, ok := sp.activeEpochs[epoch]
	if ok { // epoch already registered, load existing datahashes from datastore
		datahashes, err = sp.getDatahashesFromEpoch(ctx, epoch)
		if err != nil {
			return err
		}
	} else { // epoch not already registered
		log.Infow("registering new epoch", "epoch", epoch)
		sp.activeEpochs[epoch] = struct{}{}
	}

	oldest := sp.oldestEpoch.Load()
	if epoch < oldest {
		sp.oldestEpoch.CompareAndSwap(oldest, epoch)
	}

	datahashes = append(datahashes, h.DAH.Hash())
	return sp.saveDatahashesToEpoch(ctx, epoch, datahashes)
}

func (sp *StoragePruner) gc(ctx context.Context) error {
	for epoch := range sp.activeEpochs {
		err := sp.pruneEpoch(ctx, epoch)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sp *StoragePruner) run(ctx context.Context) {
	ticker := time.NewTicker(sp.cfg.EpochDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := sp.pruneEpoch(ctx, sp.oldestEpoch.Load())
			if err != nil {
				log.Errorw("pruning oldest epoch", "err", err)
			}
			// skip gc if there is nothing to collect after pruning oldest epoch
			if len(sp.activeEpochs) < int(float64(sp.cfg.RecencyWindow)/float64(sp.cfg.EpochDuration)) {
				continue
			}

			err = sp.gc(ctx)
			if err != nil {
				log.Errorw("gc failed", "err", err)
			}
		case <-ctx.Done():
			sp.done <- struct{}{}
			return
		}
	}
}

func (sp *StoragePruner) pruneEpoch(ctx context.Context, epoch uint64) error {
	if sp.epochIsRecent(epoch) {
		return nil
	}

	log.Infow("pruning epoch", "epoch", epoch)

	lk := &sp.stripedLocks[epoch%256]
	lk.Lock()
	defer lk.Unlock()
	datahashes, err := sp.getDatahashesFromEpoch(ctx, epoch)
	if err != nil {
		return err
	}

	for _, dh := range datahashes {
		err = sp.store.Remove(ctx, dh)
		if err != nil {
			return err
		}
	}

	delete(sp.activeEpochs, epoch)
	if epoch == sp.oldestEpoch.Load() {
		sp.updateOldestEpoch()
	}
	return nil
}

func (sp *StoragePruner) updateOldestEpoch() {
	oldest := sp.oldestEpoch.Load()
	next := oldest
	for {
		nextTime := sp.epochToTime(next).Add(sp.cfg.EpochDuration)
		if nextTime.After(time.Now()) {
			log.Warn("could not find new oldest epoch")
			break
		}
		nextEpoch := sp.calculateEpoch(nextTime)
		if _, ok := sp.activeEpochs[nextEpoch]; ok {
			sp.oldestEpoch.CompareAndSwap(oldest, nextEpoch)
			break
		}
		next = oldest + 1
	}
}

func (sp *StoragePruner) getDatahashesFromEpoch(ctx context.Context, epoch uint64) ([]share.DataHash, error) {
	var datahashes []share.DataHash
	key := datastore.NewKey(fmt.Sprintf("%d", epoch))
	val, err := sp.ds.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get datahashes for epoch %d: %w", epoch, err)
	}
	if err := json.Unmarshal(val, &datahashes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal datahashes for epoch %d: %w", epoch, err)
	}
	return datahashes, nil
}

func (sp *StoragePruner) saveDatahashesToEpoch(ctx context.Context, epoch uint64, datahashes []share.DataHash) error {
	key := datastore.NewKey(fmt.Sprintf("%d", epoch))
	// TODO: Can we avoid expensive JSON marshal/unmarshal
	bz, err := json.Marshal(datahashes)
	if err != nil {
		return fmt.Errorf("failed to marshal datahashes for epoch %d: %w", epoch, err)
	}
	err = sp.ds.Put(ctx, key, bz)
	if err != nil {
		return fmt.Errorf("failed to put datahashes for epoch %d: %w", epoch, err)
	}
	return nil
}

func (sp *StoragePruner) calculateEpoch(timestamp time.Time) uint64 {
	return uint64(timestamp.Unix() / int64(sp.cfg.EpochDuration.Seconds()))
}

func (sp *StoragePruner) epochToTime(epoch uint64) time.Time {
	return time.Unix(int64(epoch*uint64(sp.cfg.EpochDuration.Seconds())), 0)
}

func (sp *StoragePruner) epochIsRecent(epoch uint64) bool {
	return epoch >= sp.calculateEpoch(time.Now().Add(-sp.cfg.RecencyWindow))
}
