package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"
)

func TestStore_Put(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)

	p := newValidProof()
	bin, err := p.MarshalBinary()
	require.NoError(t, err)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	store := namespace.Wrap(ds, makeKey(p.Type()))
	err = put(ctx, store, string(p.HeaderHash()), bin)
	require.NoError(t, err)
}

func TestStore_GetAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)

	proof := newValidProof()
	bin, err := proof.MarshalBinary()
	require.NoError(t, err)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	proofStore := namespace.Wrap(ds, makeKey(proof.Type()))

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

	proof := newValidProof()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	store := namespace.Wrap(ds, makeKey(proof.Type()))

	proofs, err := getAll(ctx, store, proof.Type())
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrNotFound)
	require.Nil(t, proofs)
}

func Test_GetByHash(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)
	bServ := mdutils.Bserv()
	_, store := createService(t)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	badEncodingStore := namespace.Wrap(ds, makeKey(BadEncoding))
	h, err := store.GetByHeight(ctx, 1)
	require.NoError(t, err)
	faultDAH, err := generateByzantineError(ctx, t, h, bServ)
	var errByz *ipld.ErrByzantine
	require.True(t, errors.As(err, &errByz))

	p := CreateBadEncodingProof(h.Hash(), uint64(faultDAH.Height), errByz)
	bin, err := p.MarshalBinary()
	require.NoError(t, err)
	err = put(ctx, badEncodingStore, hex.EncodeToString(p.HeaderHash()), bin)
	require.NoError(t, err)

	rawData, err := getByHash(ctx, badEncodingStore, hex.EncodeToString(p.HeaderHash()))
	require.NoError(t, err)
	require.NotEmpty(t, rawData)
	befp, err := UnmarshalBEFP(rawData)
	require.NoError(t, err)
	require.NoError(t, befp.Validate(faultDAH))
}
