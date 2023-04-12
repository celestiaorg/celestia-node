package blob

import (
	"bytes"
	"context"
	"sort"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/go-header/store"
	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/minio/sha256-simd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	blob := generateBlob(t, 10)
	blob1 := generateBlob(t, 6)
	rawShares := splitBlob(t, blob, blob1)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, hstore, bs)
	newBlob, err := service.Get(ctx, 1, blob1.NamespaceID(), blob1.Commitment())
	require.NoError(t, err)
	require.Equal(t, blob1.Commitment(), newBlob.Commitment())
}

func TestService_GetSingleBlobInHeaderWithSingleNID(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	t.Cleanup(cancel)

	// TODO(@vgonkivs): add functionality to create blobs with the same nID
	blob := generateBlob(t, 12)
	nid := blob.NamespaceID()
	blob1 := generateBlob(t, 4)
	blob1.NamespaceId = nid
	blob1.commitment, err = types.CreateCommitment(&blob1.Blob)
	require.NoError(t, err)

	rawShares := splitBlob(t, blob, blob1)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, hstore, bs)
	newBlob, err := service.Get(ctx, 1, blob1.NamespaceID(), blob1.Commitment())
	require.NoError(t, err)
	require.Equal(t, blob1.Commitment(), newBlob.Commitment())

	blobs, err := service.GetAll(ctx, 1, blob.NamespaceID())
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

	blob := generateBlob(t, 16)
	blob1 := generateBlob(t, 6)
	rawShares := splitBlob(t, blob)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, hstore, bs)
	_, err = service.Get(ctx, 1, blob1.NamespaceID(), blob1.Commitment())
	require.Error(t, err)
}

func TestService_GetFailedWithInvalidCommitment(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blob := generateBlob(t, 16)
	blob1 := generateBlob(t, 6)
	rawShares := splitBlob(t, blob)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, hstore, bs)
	_, err = service.Get(ctx, 1, blob.NamespaceID(), blob1.Commitment())
	require.Error(t, err)
}

func TestService_GetAll(t *testing.T) {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	hstore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := []*Blob{generateBlob(t, 10), generateBlob(t, 6)}
	rawShares := splitBlob(t, blobs[0], blobs[1])
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, hstore, bs)
	newBlobs, err := service.GetAll(ctx, 1, blobs[0].NamespaceID(), blobs[1].NamespaceID())
	assert.Len(t, newBlobs, 2)

	sort.Slice(blobs, func(i, j int) bool {
		val := bytes.Compare(blobs[i].NamespaceID(), blobs[j].NamespaceID())
		if val > 0 {
			return false
		}
		return true
	})

	sort.Slice(newBlobs, func(i, j int) bool {
		val := bytes.Compare(blobs[i].NamespaceID(), blobs[j].NamespaceID())
		if val > 0 {
			return false
		}
		return true
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	t.Cleanup(cancel)

	blob := generateBlob(t, 16)
	rawShares := splitBlob(t, blob)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = hstore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, hstore, bs)
	proof, err := service.GetProof(ctx, 1, blob.NamespaceID(), blob.Commitment())
	require.NoError(t, err)
	assert.Equal(t, uint(proof.Len()), eds.Width()/2)

	for i, p := range *proof {
		eq := p.VerifyInclusion(sha256.New(), blob.NamespaceID(), rawShares[i*4:(i+1)*4], eds.RowRoots()[i])
		require.True(t, eq)
	}
}
