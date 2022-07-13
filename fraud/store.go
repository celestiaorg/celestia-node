package fraud

import (
	"context"
	"sort"
	"strings"

	"github.com/ipfs/go-datastore"
	q "github.com/ipfs/go-datastore/query"
)

var (
	storePrefix = "fraud"
)

// put adds a Fraud Proof to the datastore with the given key.
func put(ctx context.Context, ds datastore.Datastore, hash string, value []byte) error {
	return ds.Put(ctx, datastore.NewKey(hash), value)
}

// query allows to send any custom requests that will be needed.
func query(ctx context.Context, ds datastore.Datastore, q q.Query) ([]q.Entry, error) {
	results, err := ds.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	return results.Rest()
}

// getAll queries all Fraud Proofs by its type.
func getAll(ctx context.Context, ds datastore.Datastore, u ProofUnmarshaler) ([]Proof, error) {
	entries, err := query(ctx, ds, q.Query{})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, datastore.ErrNotFound
	}
	proofs := make([]Proof, 0)
	for _, data := range entries {
		proof, err := u(data.Value)
		if err != nil {
			continue
		}
		proofs = append(proofs, proof)
	}
	sort.Slice(proofs, func(i, j int) bool {
		return proofs[i].Height() < proofs[j].Height()
	})
	return proofs, nil
}

func makeKey(p ProofType) datastore.Key {
	return datastore.NewKey(strings.Join([]string{storePrefix, p.String()}, "/"))
}
