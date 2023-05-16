package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
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

func TestBlobService_Get(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)
	var (
		blobSize0 = 18
		blobSize1 = 14
		blobSize2 = 20
		blobSize3 = 12
	)
	blobs0 := generateBlob(t, []int{blobSize0, blobSize1}, false)
	blobs1 := generateBlob(t, []int{blobSize2, blobSize3}, true)
	service := createService(ctx, t, append(blobs0, blobs1...))
	var test = []struct {
		name           string
		doFn           func() (interface{}, error)
		expectedResult func(interface{}, error)
	}{
		{
			name: "get single blob",
			doFn: func() (interface{}, error) {
				b, err := service.Get(ctx, 1, blobs0[0].NamespaceID(), blobs0[0].Commitment())
				return []*Blob{b}, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, res)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.Len(t, blobs, 1)

				assert.Equal(t, blobs0[0].Commitment(), blobs[0].Commitment())
			},
		},
		{
			name: "get all with the same nID",
			doFn: func() (interface{}, error) {
				b, err := service.GetAll(ctx, 1, blobs1[0].NamespaceID())
				return b, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.NotEmpty(t, blobs)

				assert.Len(t, blobs, 2)

				for i := range blobs1 {
					bytes.Equal(blobs[i].Commitment(), blobs[i].Commitment())
				}
			},
		},
		{
			name: "get all with different nIDs",
			doFn: func() (interface{}, error) {
				b, err := service.GetAll(ctx, 1, blobs0[0].NamespaceID(), blobs0[1].NamespaceID())
				return b, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.NotEmpty(t, blobs)

				assert.Len(t, blobs, 2)
			},
		},
		{
			name: "get blob with incorrect commitment",
			doFn: func() (interface{}, error) {
				b, err := service.Get(ctx, 1, blobs0[0].NamespaceID(), blobs0[1].Commitment())
				return []*Blob{b}, err
			},
			expectedResult: func(res interface{}, err error) {
				require.Error(t, err)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.Empty(t, blobs[0])
			},
		},
		{
			name: "get invalid blob",
			doFn: func() (interface{}, error) {
				blob := generateBlob(t, []int{10}, false)
				b, err := service.Get(ctx, 1, blob[0].NamespaceID(), blob[0].Commitment())
				return []*Blob{b}, err
			},
			expectedResult: func(res interface{}, err error) {
				require.Error(t, err)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.Empty(t, blobs[0])
			},
		},
		{
			name: "get proof",
			doFn: func() (interface{}, error) {
				proof, err := service.GetProof(ctx, 1, blobs0[1].NamespaceID(), blobs0[1].Commitment())
				return proof, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)

				header, err := service.headerGetter(ctx, 1)
				require.NoError(t, err)

				proof, ok := res.(*Proof)
				assert.True(t, ok)

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

				rawShares, err := BlobsToShares(blobs0[1])
				require.NoError(t, err)
				verifyFn(t, rawShares, proof, blobs0[1].NamespaceID())
			},
		},
		{
			name: "verify inclusion",
			doFn: func() (interface{}, error) {
				proof, err := service.GetProof(ctx, 1, blobs0[0].NamespaceID(), blobs0[0].Commitment())
				require.NoError(t, err)
				return service.Included(ctx, 1, blobs0[0].NamespaceID(), proof, blobs0[0].Commitment())
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)
				included, ok := res.(bool)
				require.True(t, ok)
				require.True(t, included)
			},
		},
		{
			name: "verify inclusion fails with different proof",
			doFn: func() (interface{}, error) {
				proof, err := service.GetProof(ctx, 1, blobs0[1].NamespaceID(), blobs0[1].Commitment())
				require.NoError(t, err)
				return service.Included(ctx, 1, blobs0[0].NamespaceID(), proof, blobs0[0].Commitment())
			},
			expectedResult: func(res interface{}, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidProof)
				included, ok := res.(bool)
				require.True(t, ok)
				require.False(t, included)
			},
		},
		{
			name: "not included",
			doFn: func() (interface{}, error) {
				blob := generateBlob(t, []int{10}, false)
				proof, err := service.GetProof(ctx, 1, blobs0[1].NamespaceID(), blobs0[1].Commitment())
				require.NoError(t, err)
				return service.Included(ctx, 1, blob[0].NamespaceID(), proof, blob[0].Commitment())
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)
				included, ok := res.(bool)
				require.True(t, ok)
				require.False(t, included)
			},
		},
		{
			name: "count proofs for the blob",
			doFn: func() (interface{}, error) {
				proof0, err := service.GetProof(ctx, 1, blobs0[0].NamespaceID(), blobs0[0].Commitment())
				if err != nil {
					return nil, err
				}
				proof1, err := service.GetProof(ctx, 1, blobs0[1].NamespaceID(), blobs0[1].Commitment())
				if err != nil {
					return nil, err
				}
				return []*Proof{proof0, proof1}, nil
			},
			expectedResult: func(i interface{}, err error) {
				require.NoError(t, err)
				proofs, ok := i.([]*Proof)
				require.True(t, ok)

				h, err := service.headerGetter(ctx, 1)
				require.NoError(t, err)

				originalDataWidth := len(h.DAH.RowsRoots) / 2
				sizes := []int{blobSize0, blobSize1}
				for i, proof := range proofs {
					require.True(t, sizes[i]/originalDataWidth+1 == proof.Len())
				}
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			blobs, err := tt.doFn()
			tt.expectedResult(blobs, err)
		})
	}
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
	rawShares0, err := BlobsToShares(blobs[0])
	require.NoError(t, err)
	rawShares1, err := BlobsToShares(blobs[1])
	require.NoError(t, err)

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

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return headerStore.GetByHeight(ctx, height)
	}
	service := NewService(nil, getters.NewIPLDGetter(bs), fn)

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
	rawShares0, err := BlobsToShares(blobs[0])
	require.NoError(t, err)
	rawShares1, err := BlobsToShares(blobs[1])
	require.NoError(t, err)
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

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return headerStore.GetByHeight(ctx, height)
	}

	service := NewService(nil, getters.NewIPLDGetter(bs), fn)

	_, err = service.GetAll(ctx, 1, blobs[0].NamespaceID(), blobs[1].NamespaceID())
	require.NoError(t, err)
}

func createService(ctx context.Context, t *testing.T, blobs []*Blob) *Service {
	bs := mdutils.Bserv()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	headerStore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)
	rawShares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	eds, err := share.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = headerStore.Init(ctx, h)
	require.NoError(t, err)

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return headerStore.GetByHeight(ctx, height)
	}
	return NewService(nil, getters.NewIPLDGetter(bs), fn)
}
