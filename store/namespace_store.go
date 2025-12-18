package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

// namespaceStore provides persistent storage for verified NamespaceData (shares + proofs).
// It allows the node to serve verified blob data for a specific namespace without storing
// the full ODS/EDS files.
type namespaceStore struct {
	ds datastore.Batching
}

func newNamespaceStore(ds datastore.Batching) *namespaceStore {
	return &namespaceStore{ds: ds}
}

func (ns *namespaceStore) put(ctx context.Context, height uint64, data shwap.NamespaceData) error {
	if len(data) == 0 {
		return nil
	}

	buf := new(bytes.Buffer)
	_, err := data.WriteTo(buf)
	if err != nil {
		return fmt.Errorf("serializing namespace data: %w", err)
	}

	key := datastore.NewKey(fmt.Sprintf("%d", height))
	return ns.ds.Put(ctx, key, buf.Bytes())
}

func (ns *namespaceStore) get(ctx context.Context, height uint64) (shwap.NamespaceData, error) {
	key := datastore.NewKey(fmt.Sprintf("%d", height))
	val, err := ns.ds.Get(ctx, key)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var data shwap.NamespaceData
	_, err = data.ReadFrom(bytes.NewReader(val))
	if err != nil {
		return nil, fmt.Errorf("deserializing namespace data: %w", err)
	}

	return data, nil
}

func (ns *namespaceStore) delete(ctx context.Context, height uint64) error {
	key := datastore.NewKey(fmt.Sprintf("%d", height))
	return ns.ds.Delete(ctx, key)
}
