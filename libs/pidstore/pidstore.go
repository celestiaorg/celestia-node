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
func NewPeerIDStore(ds datastore.Datastore) *PeerIDStore {
	return &PeerIDStore{
		ds: namespace.Wrap(ds, storePrefix),
	}
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
