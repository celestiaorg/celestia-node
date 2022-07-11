package fraud

import (
	"context"
	"fmt"
	"strings"

	"github.com/ipfs/go-datastore"
	q "github.com/ipfs/go-datastore/query"
)

var (
	storePrefix = "fraud"
)

type ErrFraudExists struct {
	Type   ProofType
	Proofs [][]byte
}

func (e *ErrFraudExists) Error() string {
	return fmt.Sprintf("fraud: %s proof exists\n", e.Type)
}

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
func getAll(ctx context.Context, ds datastore.Datastore) ([][]byte, error) {
	entries, err := query(ctx, ds, q.Query{Orders: []q.Order{q.OrderByKeyDescending{}}})
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
