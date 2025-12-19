package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

// namespaceStore provides persistent storage for verified NamespaceData (shares + proofs).
// This is used by Pin nodes to cache namespace-specific data for fast retrieval.
type namespaceStore struct {
	ds datastore.Batching
}

func newNamespaceStore(ds datastore.Batching) *namespaceStore {
	return &namespaceStore{ds: ds}
}

func (ns *namespaceStore) put(ctx context.Context, height uint64, data shwap.NamespaceData) error {
	key := ns.heightToKey(height)

	var buf bytes.Buffer
	_, err := data.WriteTo(&buf)
	if err != nil {
		return fmt.Errorf("serializing namespace data: %w", err)
	}

	return ns.ds.Put(ctx, key, buf.Bytes())
}

func (ns *namespaceStore) get(ctx context.Context, height uint64) (shwap.NamespaceData, error) {
	key := ns.heightToKey(height)
	data, err := ns.ds.Get(ctx, key)
	if errors.Is(err, datastore.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting namespace data: %w", err)
	}

	var nd shwap.NamespaceData
	_, err = nd.ReadFrom(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("deserializing namespace data: %w", err)
	}
	return nd, nil
}

func (ns *namespaceStore) delete(ctx context.Context, height uint64) error {
	key := ns.heightToKey(height)
	return ns.ds.Delete(ctx, key)
}

func (ns *namespaceStore) heightToKey(height uint64) datastore.Key {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, height)
	return datastore.NewKey("/ns_data").ChildString(string(buf))
}

