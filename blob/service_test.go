package blob

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/blob/blobtest"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/ipld"
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

	appBlobs, err := blobtest.GenerateV0Blobs([]int{blobSize0, blobSize1}, false)
	require.NoError(t, err)
	blobs0, err := convertBlobs(appBlobs...)
	require.NoError(t, err)

	appBlobs, err = blobtest.GenerateV0Blobs([]int{blobSize2, blobSize3}, true)
	require.NoError(t, err)
	blobs1, err := convertBlobs(appBlobs...)
	require.NoError(t, err)

	service := createService(ctx, t, append(blobs0, blobs1...))
	test := []struct {
		name           string
		doFn           func() (interface{}, error)
		expectedResult func(interface{}, error)
	}{
		{
			name: "get single blob",
			doFn: func() (interface{}, error) {
				b, err := service.Get(ctx, 1, blobs0[0].Namespace(), blobs0[0].Commitment)
				return []*Blob{b}, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, res)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.Len(t, blobs, 1)

				assert.Equal(t, blobs0[0].Commitment, blobs[0].Commitment)
			},
		},
		{
			name: "get all with the same namespace",
			doFn: func() (interface{}, error) {
				return service.GetAll(ctx, 1, []share.Namespace{blobs1[0].Namespace()})
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.NotEmpty(t, blobs)

				assert.Len(t, blobs, 2)

				for i := range blobs1 {
					require.Equal(t, blobs1[i].Commitment, blobs[i].Commitment)
				}
			},
		},
		{
			name: "verify indexes",
			doFn: func() (interface{}, error) {
				b0, err := service.Get(ctx, 1, blobs0[0].Namespace(), blobs0[0].Commitment)
				require.NoError(t, err)
				b1, err := service.Get(ctx, 1, blobs0[1].Namespace(), blobs0[1].Commitment)
				require.NoError(t, err)
				b23, err := service.GetAll(ctx, 1, []share.Namespace{blobs1[0].Namespace()})
				require.NoError(t, err)
				return []*Blob{b0, b1, b23[0], b23[1]}, nil
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)
				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.NotEmpty(t, blobs)
				assert.Len(t, blobs, 4)

				sort.Slice(blobs, func(i, j int) bool {
					val := bytes.Compare(blobs[i].NamespaceId, blobs[j].NamespaceId)
					return val < 0
				})

				h, err := service.headerGetter(ctx, 1)
				require.NoError(t, err)

				resultShares, err := BlobsToShares(blobs...)
				require.NoError(t, err)
				shareOffset := 0
				for i := range blobs {
					row, col := calculateIndex(len(h.DAH.RowRoots), blobs[i].index)
					sh, err := service.shareGetter.GetShare(ctx, h, row, col)
					require.NoError(t, err)
					require.True(t, bytes.Equal(sh, resultShares[shareOffset]),
						fmt.Sprintf("issue on %d attempt. ROW:%d, COL: %d, blobIndex:%d", i, row, col, blobs[i].index),
					)
					shareOffset += shares.SparseSharesNeeded(uint32(len(blobs[i].Data)))
				}
			},
		},
		{
			name: "get all with different namespaces",
			doFn: func() (interface{}, error) {
				b, err := service.GetAll(ctx, 1, []share.Namespace{blobs0[0].Namespace(), blobs0[1].Namespace()})
				return b, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.NotEmpty(t, blobs)

				assert.Len(t, blobs, 2)
				// check the order
				require.True(t, bytes.Equal(blobs[0].Namespace(), blobs0[0].Namespace()))
				require.True(t, bytes.Equal(blobs[1].Namespace(), blobs0[1].Namespace()))
			},
		},
		{
			name: "get blob with incorrect commitment",
			doFn: func() (interface{}, error) {
				b, err := service.Get(ctx, 1, blobs0[0].Namespace(), blobs0[1].Commitment)
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
				appBlob, err := blobtest.GenerateV0Blobs([]int{10}, false)
				require.NoError(t, err)
				blob, err := convertBlobs(appBlob...)
				require.NoError(t, err)

				b, err := service.Get(ctx, 1, blob[0].Namespace(), blob[0].Commitment)
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
				proof, err := service.GetProof(ctx, 1, blobs0[1].Namespace(), blobs0[1].Commitment)
				return proof, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)

				header, err := service.headerGetter(ctx, 1)
				require.NoError(t, err)

				proof, ok := res.(*Proof)
				assert.True(t, ok)

				verifyFn := func(t *testing.T, rawShares [][]byte, proof *Proof, namespace share.Namespace) {
					for _, row := range header.DAH.RowRoots {
						to := 0
						for _, p := range *proof {
							from := to
							to = p.End() - p.Start() + from
							eq := p.VerifyInclusion(share.NewSHA256Hasher(), namespace.ToNMT(), rawShares[from:to], row)
							if eq == true {
								return
							}
						}
					}
					t.Fatal("could not prove the shares")
				}

				rawShares, err := BlobsToShares(blobs0[1])
				require.NoError(t, err)
				verifyFn(t, rawShares, proof, blobs0[1].Namespace())
			},
		},
		{
			name: "verify inclusion",
			doFn: func() (interface{}, error) {
				proof, err := service.GetProof(ctx, 1, blobs0[0].Namespace(), blobs0[0].Commitment)
				require.NoError(t, err)
				return service.Included(ctx, 1, blobs0[0].Namespace(), proof, blobs0[0].Commitment)
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
				proof, err := service.GetProof(ctx, 1, blobs0[1].Namespace(), blobs0[1].Commitment)
				require.NoError(t, err)
				return service.Included(ctx, 1, blobs0[0].Namespace(), proof, blobs0[0].Commitment)
			},
			expectedResult: func(res interface{}, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidProof)
				included, ok := res.(bool)
				require.True(t, ok)
				require.True(t, included)
			},
		},
		{
			name: "not included",
			doFn: func() (interface{}, error) {
				appBlob, err := blobtest.GenerateV0Blobs([]int{10}, false)
				require.NoError(t, err)
				blob, err := convertBlobs(appBlob...)
				require.NoError(t, err)

				proof, err := service.GetProof(ctx, 1, blobs0[1].Namespace(), blobs0[1].Commitment)
				require.NoError(t, err)
				return service.Included(ctx, 1, blob[0].Namespace(), proof, blob[0].Commitment)
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
				proof0, err := service.GetProof(ctx, 1, blobs0[0].Namespace(), blobs0[0].Commitment)
				if err != nil {
					return nil, err
				}
				proof1, err := service.GetProof(ctx, 1, blobs0[1].Namespace(), blobs0[1].Commitment)
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

				originalDataWidth := len(h.DAH.RowRoots) / 2
				sizes := []int{blobSize0, blobSize1}
				for i, proof := range proofs {
					require.True(t, sizes[i]/originalDataWidth+1 == proof.Len())
				}
			},
		},
		{
			name: "get all not found",
			doFn: func() (interface{}, error) {
				namespace := share.Namespace(tmrand.Bytes(share.NamespaceSize))
				return service.GetAll(ctx, 1, []share.Namespace{namespace})
			},
			expectedResult: func(i interface{}, err error) {
				blobs, ok := i.([]*Blob)
				require.True(t, ok)
				assert.Empty(t, blobs)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrBlobNotFound)
			},
		},
		{
			name: "marshal proof",
			doFn: func() (interface{}, error) {
				proof, err := service.GetProof(ctx, 1, blobs0[1].Namespace(), blobs0[1].Commitment)
				require.NoError(t, err)
				return json.Marshal(proof)
			},
			expectedResult: func(i interface{}, err error) {
				require.NoError(t, err)
				jsonData, ok := i.([]byte)
				require.True(t, ok)
				var proof Proof
				require.NoError(t, json.Unmarshal(jsonData, &proof))

				newProof, err := service.GetProof(ctx, 1, blobs0[1].Namespace(), blobs0[1].Commitment)
				require.NoError(t, err)
				require.NoError(t, proof.equal(*newProof))
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

// TestService_GetSingleBlobWithoutPadding creates two blobs with the same namespace
// But to satisfy the rule of eds creating, padding namespace share is placed between
// blobs. Test ensures that blob service will skip padding share and return the correct blob.
func TestService_GetSingleBlobWithoutPadding(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	t.Cleanup(cancel)

	appBlob, err := blobtest.GenerateV0Blobs([]int{9, 5}, true)
	require.NoError(t, err)
	blobs, err := convertBlobs(appBlob...)
	require.NoError(t, err)

	ns1, ns2 := blobs[0].Namespace().ToAppNamespace(), blobs[1].Namespace().ToAppNamespace()

	padding0, err := shares.NamespacePaddingShare(ns1, appconsts.ShareVersionZero)
	require.NoError(t, err)
	padding1, err := shares.NamespacePaddingShare(ns2, appconsts.ShareVersionZero)
	require.NoError(t, err)
	rawShares0, err := BlobsToShares(blobs[0])
	require.NoError(t, err)
	rawShares1, err := BlobsToShares(blobs[1])
	require.NoError(t, err)

	rawShares := make([][]byte, 0)
	rawShares = append(rawShares, append(rawShares0, padding0.ToBytes())...)
	rawShares = append(rawShares, append(rawShares1, padding1.ToBytes())...)

	bs := ipld.NewMemBlockservice()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	headerStore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)
	eds, err := ipld.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = headerStore.Init(ctx, h)
	require.NoError(t, err)

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return headerStore.GetByHeight(ctx, height)
	}
	fn2 := func(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
		return nil, fmt.Errorf("not implemented")
	}
	service := NewService(nil, getters.NewIPLDGetter(bs), fn, fn2)

	newBlob, err := service.Get(ctx, 1, blobs[1].Namespace(), blobs[1].Commitment)
	require.NoError(t, err)
	assert.Equal(t, newBlob.Commitment, blobs[1].Commitment)

	resultShares, err := BlobsToShares(newBlob)
	require.NoError(t, err)
	row, col := calculateIndex(len(h.DAH.RowRoots), newBlob.index)
	sh, err := service.shareGetter.GetShare(ctx, h, row, col)
	require.NoError(t, err)

	assert.Equal(t, sh, resultShares[0])
}

func TestService_Get(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	sizes := []int{1, 6, 3, 2, 4, 6, 8, 2, 15, 17}

	appBlobs, err := blobtest.GenerateV0Blobs(sizes, true)
	require.NoError(t, err)
	blobs, err := convertBlobs(appBlobs...)
	require.NoError(t, err)

	service := createService(ctx, t, blobs)

	h, err := service.headerGetter(ctx, 1)
	require.NoError(t, err)

	resultShares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	shareOffset := 0

	for i, blob := range blobs {
		b, err := service.Get(ctx, 1, blob.Namespace(), blob.Commitment)
		require.NoError(t, err)
		assert.Equal(t, b.Commitment, blob.Commitment)

		row, col := calculateIndex(len(h.DAH.RowRoots), b.index)
		sh, err := service.shareGetter.GetShare(ctx, h, row, col)
		require.NoError(t, err)

		assert.Equal(t, sh, resultShares[shareOffset], fmt.Sprintf("issue on %d attempt", i))
		shareOffset += shares.SparseSharesNeeded(uint32(len(blob.Data)))
	}
}

// TestService_GetAllWithoutPadding it retrieves all blobs under the given namespace:
// the amount of the blobs is known and equal to 5. Then it ensures that each blob has a correct
// index inside the eds by requesting share and comparing them.
func TestService_GetAllWithoutPadding(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	appBlob, err := blobtest.GenerateV0Blobs([]int{9, 5, 15, 4, 24}, true)
	require.NoError(t, err)
	blobs, err := convertBlobs(appBlob...)
	require.NoError(t, err)

	var (
		ns        = blobs[0].Namespace().ToAppNamespace()
		rawShares = make([][]byte, 0)
	)

	padding, err := shares.NamespacePaddingShare(ns, appconsts.ShareVersionZero)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		sh, err := BlobsToShares(blobs[i])
		require.NoError(t, err)
		rawShares = append(rawShares, append(sh, padding.ToBytes())...)
	}

	sh, err := BlobsToShares(blobs[2])
	require.NoError(t, err)
	rawShares = append(rawShares, append(sh, padding.ToBytes(), padding.ToBytes())...)

	sh, err = BlobsToShares(blobs[3])
	require.NoError(t, err)
	rawShares = append(rawShares, append(sh, padding.ToBytes(), padding.ToBytes(), padding.ToBytes())...)

	sh, err = BlobsToShares(blobs[4])
	require.NoError(t, err)
	rawShares = append(rawShares, sh...)

	bs := ipld.NewMemBlockservice()
	require.NoError(t, err)
	eds, err := ipld.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return h, nil
	}
	fn2 := func(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
		return nil, fmt.Errorf("not implemented")
	}
	service := NewService(nil, getters.NewIPLDGetter(bs), fn, fn2)

	newBlobs, err := service.GetAll(ctx, 1, []share.Namespace{blobs[0].Namespace()})
	require.NoError(t, err)
	assert.Equal(t, len(newBlobs), len(blobs))

	resultShares, err := BlobsToShares(newBlobs...)
	require.NoError(t, err)

	shareOffset := 0
	for i, blob := range newBlobs {
		require.True(t, blobs[i].compareCommitments(blob.Commitment))

		row, col := calculateIndex(len(h.DAH.RowRoots), blob.index)
		sh, err := service.shareGetter.GetShare(ctx, h, row, col)
		require.NoError(t, err)

		assert.Equal(t, sh, resultShares[shareOffset])
		shareOffset += shares.SparseSharesNeeded(uint32(len(blob.Data)))
	}
}

func TestAllPaddingSharesInEDS(t *testing.T) {
	nid, err := share.NewBlobNamespaceV0(tmrand.Bytes(7))
	require.NoError(t, err)
	padding, err := shares.NamespacePaddingShare(nid.ToAppNamespace(), appconsts.ShareVersionZero)
	require.NoError(t, err)

	rawShares := make([]share.Share, 16)
	for i := 0; i < 16; i++ {
		rawShares[i] = padding.ToBytes()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	bs := ipld.NewMemBlockservice()
	require.NoError(t, err)
	eds, err := ipld.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return h, nil
	}
	fn2 := func(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
		return nil, fmt.Errorf("not implemented")
	}
	service := NewService(nil, getters.NewIPLDGetter(bs), fn, fn2)
	_, err = service.GetAll(ctx, 1, []share.Namespace{nid})
	require.Error(t, err)
}

func TestSkipPaddingsAndRetrieveBlob(t *testing.T) {
	nid, err := share.NewBlobNamespaceV0(tmrand.Bytes(7))
	require.NoError(t, err)
	padding, err := shares.NamespacePaddingShare(nid.ToAppNamespace(), appconsts.ShareVersionZero)
	require.NoError(t, err)

	rawShares := make([]share.Share, 0, 64)
	for i := 0; i < 58; i++ {
		rawShares = append(rawShares, padding.ToBytes())
	}

	appBlob, err := blobtest.GenerateV0Blobs([]int{6}, true)
	require.NoError(t, err)
	appBlob[0].NamespaceVersion = nid[0]
	appBlob[0].NamespaceID = nid[1:]

	blobs, err := convertBlobs(appBlob...)
	require.NoError(t, err)
	sh, err := BlobsToShares(blobs[0])
	require.NoError(t, err)

	rawShares = append(rawShares, sh...)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	bs := ipld.NewMemBlockservice()
	require.NoError(t, err)
	eds, err := ipld.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return h, nil
	}
	fn2 := func(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
		return nil, fmt.Errorf("not implemented")
	}
	service := NewService(nil, getters.NewIPLDGetter(bs), fn, fn2)
	newBlob, err := service.GetAll(ctx, 1, []share.Namespace{nid})
	require.NoError(t, err)
	require.Len(t, newBlob, 1)
	require.True(t, newBlob[0].compareCommitments(blobs[0].Commitment))
}

// BenchmarkGetByCommitment-12    	    1869	    571663 ns/op	 1085371 B/op	    6414 allocs/op
func BenchmarkGetByCommitment(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	b.Cleanup(cancel)
	appBlobs, err := blobtest.GenerateV0Blobs([]int{32, 32}, true)
	require.NoError(b, err)

	blobs, err := convertBlobs(appBlobs...)
	require.NoError(b, err)

	service := createService(ctx, b, blobs)
	indexer := &parser{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.ReportAllocs()
		indexer.reset()
		indexer.verifyFn = func(blob *Blob) bool {
			return blob.compareCommitments(blobs[1].Commitment)
		}

		_, _, err = service.retrieve(
			ctx, 1, blobs[1].Namespace(), indexer,
		)
		require.NoError(b, err)
	}
}

func createService(ctx context.Context, t testing.TB, blobs []*Blob) *Service {
	bs := ipld.NewMemBlockservice()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	headerStore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)
	rawShares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	eds, err := ipld.AddShares(ctx, rawShares, bs)
	require.NoError(t, err)

	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	err = headerStore.Init(ctx, h)
	require.NoError(t, err)

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return headerStore.GetByHeight(ctx, height)
	}
	fn2 := func(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
		return nil, fmt.Errorf("not implemented")
	}
	return NewService(nil, getters.NewIPLDGetter(bs), fn, fn2)
}
