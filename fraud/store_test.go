package fraud

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/ipld"
)

func TestStore_Put(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)
	bServ := mdutils.Bserv()
	_, store := createService(t)
	h, err := store.GetByHeight(ctx, 1)
	require.NoError(t, err)

	faultDAH, err := generateByzantineError(ctx, t, h, bServ)
	var errByz *ipld.ErrByzantine
	require.True(t, errors.As(err, &errByz))
	p := CreateBadEncodingProof([]byte("hash"), uint64(faultDAH.Height), errByz)
	bin, err := p.MarshalBinary()
	require.NoError(t, err)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	badEncodingStore := namespace.Wrap(ds, makeKey(BadEncoding))
	err = put(ctx, badEncodingStore, string(p.HeaderHash()), bin)
	require.NoError(t, err)
}

func TestStore_GetAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)
	bServ := mdutils.Bserv()
	_, store := createService(t)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	badEncodingStore := namespace.Wrap(ds, makeKey(BadEncoding))
	faultHeaders := make([]*header.ExtendedHeader, 0)
	for i := 0; i < 3; i++ {
		h, err := store.GetByHeight(ctx, uint64(i+1))
		require.NoError(t, err)
		faultDAH, err := generateByzantineError(ctx, t, h, bServ)
		var errByz *ipld.ErrByzantine
		require.True(t, errors.As(err, &errByz))

		p := CreateBadEncodingProof(h.Hash(), uint64(faultDAH.Height), errByz)
		bin, err := p.MarshalBinary()
		require.NoError(t, err)
		err = put(ctx, badEncodingStore, string(p.HeaderHash()), bin)
		require.NoError(t, err)
		faultHeaders = append(faultHeaders, faultDAH)
	}
	befp, err := getAll(ctx, badEncodingStore, BadEncoding)
	require.NoError(t, err)
	require.NotEmpty(t, befp)
	for i := 0; i < len(befp); i++ {
		require.NoError(t, befp[i].Validate(faultHeaders[i]))
	}
}

func Test_GetAllFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	badEncodingStore := namespace.Wrap(ds, makeKey(BadEncoding))

	proofs, err := getAll(ctx, badEncodingStore, BadEncoding)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrNotFound)
	require.Nil(t, proofs)
}
