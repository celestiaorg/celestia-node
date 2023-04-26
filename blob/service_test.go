package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"sort"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/getters"
)

func TestService_GetSingleBlob(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{10, 6}, false)
	service := createService(ctx, t, blobs)

	newBlob, err := service.Get(ctx, 1, blobs[1].NamespaceID(), blobs[1].Commitment())
	require.NoError(t, err)
	require.Equal(t, blobs[1].Commitment(), newBlob.Commitment())
}

// TestService_GetSingleBlobInHeaderWithSingleNID creates two blobs with the same nID
// and ensures that blob service could return the correct blob by its commitment.
func TestService_GetSingleBlobInHeaderWithSingleNID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{12, 4}, true)
	service := createService(ctx, t, blobs)

	newBlob, err := service.Get(ctx, 1, blobs[1].NamespaceID(), blobs[1].Commitment())
	require.NoError(t, err)
	require.Equal(t, blobs[1].Commitment(), newBlob.Commitment())

	blobs, err = service.GetAll(ctx, 1, blobs[0].NamespaceID())
	require.NoError(t, err)
	assert.Len(t, blobs, 2)
}

// TestService_GetSingleBlobWithoutPadding creates two blobs with the same nID
// But to satisfy the rule of eds creating, padding namespace share is placed between
// blobs. Test ensures that blob service will skip padding share and return the correct blob.
func TestService_GetSingleBlobWithoutPadding(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{9, 5}, true)

	padding0 := shares.NamespacePaddingShare(blobs[0].NamespaceID())
	padding1 := shares.NamespacePaddingShare(blobs[1].NamespaceID())
	rawShares0 := splitBlob(t, blobs[0])
	rawShares1 := splitBlob(t, blobs[1])

	rawShares := make([][]byte, 0)
	rawShares = append(rawShares, append(rawShares0, padding0)...)
	rawShares = append(rawShares, append(rawShares1, padding1)...)

	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	headerStore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = headerStore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, getters.NewIPLDGetter(bs), headerStore, headerStore)

	newBlob, err := service.Get(ctx, 1, blobs[1].NamespaceID(), blobs[1].Commitment())
	require.NoError(t, err)
	assert.Equal(t, newBlob.Commitment(), blobs[1].Commitment())
}

func TestService_GetAllWithoutPadding(t *testing.T) {
	// TODO(@vgonkivs): remove skip once ParseShares will skip padding shares
	// https://github.com/celestiaorg/celestia-app/issues/1649
	t.Skip()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{9, 5}, true)

	padding0 := shares.NamespacePaddingShare(blobs[0].NamespaceID())
	padding1 := shares.NamespacePaddingShare(blobs[1].NamespaceID())
	rawShares0 := splitBlob(t, blobs[0])
	rawShares1 := splitBlob(t, blobs[1])
	rawShares := make([][]byte, 0)

	// create shares in correct order with padding shares
	if bytes.Compare(blobs[0].NamespaceID(), blobs[1].NamespaceID()) <= 0 {
		rawShares = append(rawShares, append(rawShares0, padding0)...)
		rawShares = append(rawShares, append(rawShares1, padding1)...)
	} else {
		rawShares = append(rawShares, append(rawShares1, padding1)...)
		rawShares = append(rawShares, append(rawShares0, padding0)...)
	}

	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	headerStore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = headerStore.Init(ctx, h)
	require.NoError(t, err)

	service := NewService(nil, getters.NewIPLDGetter(bs), headerStore, headerStore)

	_, err = service.GetAll(ctx, 1, blobs[0].NamespaceID(), blobs[1].NamespaceID())
	require.NoError(t, err)
}

func TestService_GetFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{16, 6}, false)

	service := createService(ctx, t, []*Blob{blobs[0]})
	_, err := service.Get(ctx, 1, blobs[1].NamespaceID(), blobs[1].Commitment())
	require.Error(t, err)
}

func TestService_GetFailedWithInvalidCommitment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{10, 6}, false)

	service := createService(ctx, t, blobs)
	_, err := service.Get(ctx, 1, blobs[0].NamespaceID(), blobs[1].Commitment())
	require.Error(t, err)
}

func TestService_GetAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{10, 6}, false)

	service := createService(ctx, t, blobs)

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	blobs := generateBlob(t, []int{10, 6}, false)
	service := createService(ctx, t, blobs)

	proof, err := service.GetProof(ctx, 1, blobs[0].NamespaceID(), blobs[0].Commitment())
	require.NoError(t, err)

	header, err := service.getByHeight(ctx, 1)
	require.NoError(t, err)

	verifyFn := func(t *testing.T, rawShares [][]byte, proof *Proof, nID namespace.ID) {
		for _, row := range header.DAH.RowsRoots {
			to := 0
			for _, p := range *proof {
				from := to
				to = p.End() - p.Start() + from
				eq := p.VerifyInclusion(sha256.New(), nID, rawShares[from:to], row)
				if eq == true {
					return
				}
			}
		}
		t.Fatal("could not prove the shares")
	}

	rawShares := splitBlob(t, blobs[0])
	verifyFn(t, rawShares, proof, blobs[0].NamespaceID())

	proof, err = service.GetProof(ctx, 1, blobs[1].NamespaceID(), blobs[1].Commitment())
	require.NoError(t, err)

	rawShares = splitBlob(t, blobs[1])
	verifyFn(t, rawShares, proof, blobs[1].NamespaceID())
}

func createService(ctx context.Context, t *testing.T, blobs []*Blob) *Service {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	headerStore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)
	rawShares := splitBlob(t, blobs...)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = headerStore.Init(ctx, h)
	require.NoError(t, err)
	return NewService(nil, getters.NewIPLDGetter(bs), headerStore, headerStore)
}
