package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

var nsStorePrefix = datastore.NewKey("ns_data")

// namespaceStore provides persistent storage for verified NamespaceData (shares + proofs).
// This is used by Pin nodes to cache namespace-specific data for fast retrieval.
type namespaceStore struct {
	ds datastore.Batching
}

func newNamespaceStore(ds datastore.Batching) *namespaceStore {
	return &namespaceStore{ds: ds}
}

func (s *namespaceStore) put(ctx context.Context, height uint64, data shwap.NamespaceData) error {
	key := s.heightToKey(height)

	var buf bytes.Buffer
	_, err := data.WriteTo(&buf)
	if err != nil {
		return fmt.Errorf("serializing namespace data: %w", err)
	}

	return s.ds.Put(ctx, key, buf.Bytes())
}

func (s *namespaceStore) get(ctx context.Context, height uint64) (shwap.NamespaceData, error) {
	key := s.heightToKey(height)
	data, err := s.ds.Get(ctx, key)
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

func (s *namespaceStore) delete(ctx context.Context, height uint64) error {
	key := s.heightToKey(height)
	return s.ds.Delete(ctx, key)
}

func (s *namespaceStore) heightToKey(height uint64) datastore.Key {
	return nsStorePrefix.ChildString(strconv.FormatUint(height, 10))
}

