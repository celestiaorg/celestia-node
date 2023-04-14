package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
)

func TestService_GetSingleBlob(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{10, 6}, false)
	rawShares := splitBlob(t, blobs...)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, bs, hstore, hstore)
	newBlob, err := service.Get(ctx, 1, blobs[1].NamespaceID(), blobs[1].Commitment())
	require.NoError(t, err)
	require.Equal(t, blobs[1].Commitment(), newBlob.Commitment())
}

func TestService_GetSingleBlobInHeaderWithSingleNID(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	// TODO(@vgonkivs): add functionality to create blobs with the same nID
	blobs := generateBlob(t, []int{12, 4}, true)
	require.NoError(t, err)

	rawShares := splitBlob(t, blobs...)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, bs, hstore, hstore)
	newBlob, err := service.Get(ctx, 1, blobs[1].NamespaceID(), blobs[1].Commitment())
	require.NoError(t, err)
	require.Equal(t, blobs[1].Commitment(), newBlob.Commitment())

	blobs, err = service.GetAll(ctx, 1, blobs[0].NamespaceID())
	require.NoError(t, err)
	assert.Len(t, blobs, 2)
}

func TestService_GetFailed(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{16, 6}, false)
	rawShares := splitBlob(t, blobs[0])
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, bs, hstore, hstore)
	_, err = service.Get(ctx, 1, blobs[1].NamespaceID(), blobs[1].Commitment())
	require.Error(t, err)
}

func TestService_GetFailedWithInvalidCommitment(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{10, 6}, false)
	rawShares := splitBlob(t, blobs...)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, bs, hstore, hstore)
	_, err = service.Get(ctx, 1, blobs[0].NamespaceID(), blobs[1].Commitment())
	require.Error(t, err)
}

func TestService_GetAll(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{10, 6}, false)
	rawShares := splitBlob(t, blobs...)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, bs, hstore, hstore)

	newBlobs, err := service.GetAll(ctx, 1, blobs[0].NamespaceID(), blobs[1].NamespaceID())
	require.NoError(t, err)
	assert.Len(t, newBlobs, 2)

	sort.Slice(blobs, func(i, j int) bool {
		val := bytes.Compare(blobs[i].NamespaceID(), blobs[j].NamespaceID())
		return val <= 0
	})

	sort.Slice(newBlobs, func(i, j int) bool {
		val := bytes.Compare(blobs[i].NamespaceID(), blobs[j].NamespaceID())
		return val <= 0
	})

	for i := range blobs {
		bytes.Equal(blobs[i].Commitment(), newBlobs[i].Commitment())
	}
}

func TestService_GetProof(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blob := generateBlob(t, []int{10, 6}, false)
	rawShares := splitBlob(t, blob...)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, bs, hstore, hstore)
	proof, err := service.GetProof(ctx, 1, blob[0].NamespaceID(), blob[0].Commitment())
	require.NoError(t, err)

	verifyFn := func(t *testing.T, rawShares [][]byte, proof *Proof, nID namespace.ID) {
		to := 0
		for i, p := range *proof {
			from := to
			to = p.proof.End() - p.proof.Start() + from
			eq := p.proof.VerifyInclusion(sha256.New(), nID, rawShares[from:to], eds.RowRoots()[p.rowIndex])
			require.Truef(t, eq, fmt.Sprintf("row:%d,start:%d,end:%d", i, from, to))
		}
	}

	rawShares = splitBlob(t, blob[0])
	verifyFn(t, rawShares, proof, blob[0].NamespaceID())

	proof, err = service.GetProof(ctx, 1, blob[1].NamespaceID(), blob[1].Commitment())
	require.NoError(t, err)

	rawShares = splitBlob(t, blob[1])
	verifyFn(t, rawShares, proof, blob[1].NamespaceID())
}
