package fraud

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	q "github.com/ipfs/go-datastore/query"
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

// getAll queries all Fraud Proofs by their type.
func getAll(ctx context.Context, ds datastore.Datastore, proofType ProofType) ([]Proof, error) {
	entries, err := query(ctx, ds, q.Query{})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, datastore.ErrNotFound
	}
	proofs := make([]Proof, 0)
	for _, data := range entries {
		proof, err := Unmarshal(proofType, data.Value)
		if err != nil {
			if errors.Is(err, &errNoUnmarshaler{}) {
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

func initStore(topic ProofType, ds datastore.Datastore) datastore.Datastore {
	return namespace.Wrap(ds, makeKey(topic))
}

func makeKey(topic ProofType) datastore.Key {
	return datastore.NewKey(fmt.Sprintf("%s/%s", storePrefix, topic))
}
