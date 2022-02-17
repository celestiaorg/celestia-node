package das

import (
	"encoding/binary"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
)

var (
	storePrefix   = datastore.NewKey("das")
	checkpointKey = datastore.NewKey("checkpoint")
)

// NewCheckpointStore wraps the given datastore.Datastore with
// the `das` prefix.
func NewCheckpointStore(ds datastore.Datastore) datastore.Datastore {
	return namespace.Wrap(ds, storePrefix)

}

// loadCheckpoint // TODO @renaynay: document
//
// 1. gets checkpoint from disk
// 1a. if there is no checkpoint, you start from `GetByHeight` of 1
// 1b. if there is a checkpoint, continue
func loadCheckpoint(ds datastore.Datastore) (int64, error) {
	checkpoint, err := ds.Get(checkpointKey)
	if err != nil {
		// if no checkpoint was found, return checkpoint as
		// 0 since DASer begins sampling on checkpoint+1
		if err == datastore.ErrNotFound {
			log.Debug("checkpoint not found, starting sampling at block height 1")
			return 0, nil
		}

		return 0, err
	}
	return int64(binary.BigEndian.Uint64(checkpoint)), err
}

// TODO @renaynay: document

// storeCheckpoint stores the given DAS checkpoint to disk.
func storeCheckpoint(ds datastore.Datastore, checkpoint int64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(checkpoint))

	return ds.Put(checkpointKey, buf)
}

