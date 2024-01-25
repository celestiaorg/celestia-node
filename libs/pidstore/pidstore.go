package pidstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	storePrefix = datastore.NewKey("pidstore")
	peersKey    = datastore.NewKey("peers")

	log = logging.Logger("pidstore")
)

// PeerIDStore is used to store/load peers to/from disk.
type PeerIDStore struct {
	ds datastore.Datastore
}

// NewPeerIDStore creates a new peer ID store backed by the given datastore.
func NewPeerIDStore(ctx context.Context, ds datastore.Datastore) (*PeerIDStore, error) {
	pidstore := &PeerIDStore{
		ds: namespace.Wrap(ds, storePrefix),
	}

	// check if pidstore is already initialized, and if not,
	// initialize the pidstore
	exists, err := pidstore.ds.Has(ctx, peersKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		return pidstore, pidstore.Put(ctx, []peer.ID{})
	}

	// if pidstore exists, ensure its contents are uncorrupted
	_, err = pidstore.Load(ctx)
	if err != nil {
		log.Warn("pidstore: corrupted pidstore detected, resetting...", "err", err)
		return pidstore, pidstore.reset(ctx)
	}

	return pidstore, nil
}

// Load loads the peers from datastore and returns them.
func (p *PeerIDStore) Load(ctx context.Context) ([]peer.ID, error) {
	log.Debug("Loading peers")

	bin, err := p.ds.Get(ctx, peersKey)
	if err != nil {
		return nil, fmt.Errorf("pidstore: loading peers from datastore: %w", err)
	}

	var peers []peer.ID
	err = json.Unmarshal(bin, &peers)
	if err != nil {
		return nil, fmt.Errorf("pidstore: unmarshalling peer IDs: %w", err)
	}

	log.Infow("Loaded peers from disk", "amount", len(peers))
	return peers, nil
}

// Put persists the given peer IDs to the datastore.
func (p *PeerIDStore) Put(ctx context.Context, peers []peer.ID) error {
	log.Debugw("Persisting peers to disk", "amount", len(peers))

	bin, err := json.Marshal(peers)
	if err != nil {
		return fmt.Errorf("pidstore: marshal peerlist: %w", err)
	}

	if err = p.ds.Put(ctx, peersKey, bin); err != nil {
		return fmt.Errorf("pidstore: error writing to datastore: %w", err)
	}

	log.Infow("Persisted peers successfully", "amount", len(peers))
	return nil
}

// reset resets the pidstore in case of corruption.
func (p *PeerIDStore) reset(ctx context.Context) error {
	log.Warn("pidstore: resetting the pidstore...")
	err := p.ds.Delete(ctx, peersKey)
	if err != nil {
		return fmt.Errorf("pidstore: error resetting datastore: %w", err)
	}

	return p.Put(ctx, []peer.ID{})
}
