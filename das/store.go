package das

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
)

var (
	storePrefix   = datastore.NewKey("das")
	checkpointKey = datastore.NewKey("checkpoint")
)

// The checkpointStore stores/loads the DASer's checkpoint to/from
// disk using the checkpointKey. The checkpoint is stored as a struct
// representation of the latest successfully DASed state.
type checkpointStore struct {
	datastore.Datastore
	done
}

// newCheckpointStore wraps the given datastore.Datastore with the `das` prefix.
func newCheckpointStore(ds datastore.Datastore) checkpointStore {
	return checkpointStore{
		namespace.Wrap(ds, storePrefix),
		newDone("checkpoint store")}
}

// load loads the DAS checkpoint from disk and returns it.
func (s *checkpointStore) load(ctx context.Context) (checkpoint, error) {
	bs, err := s.Get(ctx, checkpointKey)
	if err != nil {
		return checkpoint{}, err
	}

	cp := checkpoint{}
	err = json.Unmarshal(bs, &cp)
	return cp, err
}

// checkpointStore stores the given DAS checkpoint to disk.
func (s *checkpointStore) store(ctx context.Context, cp checkpoint) error {
	// checkpointStore latest DASed checkpoint to disk here to ensure that if DASer is not yet
	// fully caught up to network head, it will resume DASing from this checkpoint
	// up to current network head
	bs, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	if err = s.Put(ctx, checkpointKey, bs); err != nil {
		return err
	}

	log.Info("stored checkpoint to disk: ", cp.String())
	return nil
}

// runBackgroundStore periodically saves current sampling state in case of DASer force quit before
// being able to store state on exit. The routine can be disabled by passing storeInterval = 0.
func (s *checkpointStore) runBackgroundStore(
	ctx context.Context,
	storeInterval time.Duration,
	getCheckpoint func(ctx context.Context) (checkpoint, error)) {
	defer s.indicateDone()

	// runBackgroundStore could be disabled by setting storeInterval = 0
	if storeInterval == 0 {
		log.Info("DASer background checkpointStore is disabled")
		return
	}

	ticker := time.NewTicker(storeInterval)
	defer ticker.Stop()

	var prev uint64
	for {
		// blocked by ticker to perform storing only once in a period
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}

		cp, err := getCheckpoint(ctx)
		if err != nil {
			log.Debug("DASer coordinator checkpoint is unavailable")
			continue
		}
		if cp.SampleFrom > prev {
			if err = s.store(ctx, cp); err != nil {
				log.Errorw("storing checkpoint to disk", "err", err)
			}
			prev = cp.SampleFrom
		}
	}
}
