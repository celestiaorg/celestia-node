package fraud

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"
)

func TestStore_Put(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)

	p := newMockProof(true)
	bin, err := p.MarshalBinary()
	require.NoError(t, err)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	store := namespace.Wrap(ds, makeKey(p.Name()))
	err = put(ctx, store, string(p.HeaderHash()), bin)
	require.NoError(t, err)
}

func TestStore_GetAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)

	proof := newMockProof(true)
	bin, err := proof.MarshalBinary()
	require.NoError(t, err)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	proofStore := namespace.Wrap(ds, makeKey(proof.Name()))

	err = put(ctx, proofStore, string(proof.HeaderHash()), bin)
	require.NoError(t, err)

	proofs, err := getAll(ctx, proofStore, proof.Type())
	require.NoError(t, err)
	require.NotEmpty(t, proofs)
	require.NoError(t, proof.Validate(nil))

}

func Test_GetAllFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)

	proof := newMockProof(true)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	store := namespace.Wrap(ds, makeKey(proof.Name()))

	proofs, err := getAll(ctx, store, proof.Type())
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrNotFound)
	require.Nil(t, proofs)
}
