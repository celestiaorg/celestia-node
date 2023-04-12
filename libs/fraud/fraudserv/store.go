package fraudserv

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	q "github.com/ipfs/go-datastore/query"

	"github.com/celestiaorg/celestia-node/libs/fraud"
)

var (
	storePrefix = "fraud"
)

// put adds a Fraud Proof to the datastore with the given hash as the key.
func put(ctx context.Context, ds datastore.Datastore, hash string, value []byte) error {
	return ds.Put(ctx, datastore.NewKey(hash), value)
}

// query performs a custom query on the given datastore.
func query(ctx context.Context, ds datastore.Datastore, q q.Query) ([]q.Entry, error) {
	results, err := ds.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	return results.Rest()
}

// getByHash fetches a fraud proof by its hash from local storage.
func getByHash(ctx context.Context, ds datastore.Datastore, hash string) ([]byte, error) {
	return ds.Get(ctx, datastore.NewKey(hash))
}

// getAll queries all Fraud Proofs by their type.
func getAll(ctx context.Context, ds datastore.Datastore, proofType fraud.ProofType) ([]fraud.Proof, error) {
	entries, err := query(ctx, ds, q.Query{})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, datastore.ErrNotFound
	}
	proofs := make([]fraud.Proof, 0)
	for _, data := range entries {
		proof, err := fraud.Unmarshal(proofType, data.Value)
		if err != nil {
			if errors.Is(err, &fraud.ErrNoUnmarshaler{}) {
				return nil, err
			}
			log.Warn(err)
			continue
		}
		proofs = append(proofs, proof)
	}
	sort.Slice(proofs, func(i, j int) bool {
		return proofs[i].Height() < proofs[j].Height()
	})
	return proofs, nil
}

func initStore(topic fraud.ProofType, ds datastore.Datastore) datastore.Datastore {
	return namespace.Wrap(ds, makeKey(topic))
}

func makeKey(topic fraud.ProofType) datastore.Key {
	return datastore.NewKey(fmt.Sprintf("%s/%s", storePrefix, topic))
}
