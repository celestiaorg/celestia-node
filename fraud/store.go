package fraud

import (
	"context"
	"errors"
	"strings"

	"github.com/ipfs/go-datastore"
	q "github.com/ipfs/go-datastore/query"
)

var (
	storePrefix = "fraud"
	// ErrFraudStoreNotEmpty will be thrown if a fraud store holds a Fraud Proof.
	ErrFraudStoreNotEmpty = errors.New("fraud store is not empty")
)

// put adds a Fraud Proof to the datastore with the given key.
func put(ctx context.Context, ds datastore.Datastore, key datastore.Key, value []byte) error {
	return ds.Put(ctx, key, value)
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
func getAll(ctx context.Context, ds datastore.Datastore) ([][]byte, error) {
	entries, err := query(ctx, ds, q.Query{})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, datastore.ErrNotFound
	}
	proofs := make([][]byte, 0, len(entries))
	for _, val := range entries {
		proofs = append(proofs, val.Value)
	}

	return proofs, nil
}

func makeKey(p ProofType) datastore.Key {
	return datastore.NewKey(strings.Join([]string{storePrefix, p.String()}, "/"))
}
