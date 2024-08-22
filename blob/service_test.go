package blob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	pkgproof "github.com/celestiaorg/celestia-app/v2/pkg/proof"
	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"
	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/inclusion"
	squarens "github.com/celestiaorg/go-square/namespace"
	appshares "github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/blob/blobtest"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters/mock"
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
	blobsWithDiffNamespaces, err := convertBlobs(appBlobs...)
	require.NoError(t, err)

	appBlobs, err = blobtest.GenerateV0Blobs([]int{blobSize2, blobSize3}, true)
	require.NoError(t, err)
	blobsWithSameNamespace, err := convertBlobs(appBlobs...)
	require.NoError(t, err)

	blobs := slices.Concat(blobsWithDiffNamespaces, blobsWithSameNamespace)
	shares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	service := createService(ctx, t, shares)
	test := []struct {
		name           string
		doFn           func() (interface{}, error)
		expectedResult func(interface{}, error)
	}{
		{
			name: "get single blob",
			doFn: func() (interface{}, error) {
				b, err := service.Get(ctx, 1,
					blobsWithDiffNamespaces[0].Namespace(),
					blobsWithDiffNamespaces[0].Commitment,
				)
				return []*Blob{b}, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, res)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.Len(t, blobs, 1)

				assert.Equal(t, blobsWithDiffNamespaces[0].Commitment, blobs[0].Commitment)
			},
		},
		{
			name: "get all with the same namespace",
			doFn: func() (interface{}, error) {
				return service.GetAll(ctx, 1, []share.Namespace{blobsWithSameNamespace[0].Namespace()})
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.NotEmpty(t, blobs)

				assert.Len(t, blobs, 2)

				for i := range blobsWithSameNamespace {
					require.Equal(t, blobsWithSameNamespace[i].Commitment, blobs[i].Commitment)
				}
			},
		},
		{
			name: "verify indexes",
			doFn: func() (interface{}, error) {
				b0, err := service.Get(ctx, 1,
					blobsWithDiffNamespaces[0].Namespace(),
					blobsWithDiffNamespaces[0].Commitment,
				)
				require.NoError(t, err)
				b1, err := service.Get(ctx, 1,
					blobsWithDiffNamespaces[1].Namespace(),
					blobsWithDiffNamespaces[1].Commitment,
				)
				require.NoError(t, err)
				b23, err := service.GetAll(ctx, 1, []share.Namespace{blobsWithSameNamespace[0].Namespace()})
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
					shareOffset += appshares.SparseSharesNeeded(uint32(len(blobs[i].Data)))
				}
			},
		},
		{
			name: "get all with different namespaces",
			doFn: func() (interface{}, error) {
				nid, err := share.NewBlobNamespaceV0(tmrand.Bytes(7))
				require.NoError(t, err)
				b, err := service.GetAll(ctx, 1,
					[]share.Namespace{
						blobsWithDiffNamespaces[0].Namespace(), nid,
						blobsWithDiffNamespaces[1].Namespace(),
					},
				)
				return b, err
			},
			expectedResult: func(res interface{}, err error) {
				require.NoError(t, err)

				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.NotEmpty(t, blobs)
				assert.Len(t, blobs, 2)
				// check the order
				require.True(t, bytes.Equal(blobs[0].Namespace(), blobsWithDiffNamespaces[0].Namespace()))
				require.True(t, bytes.Equal(blobs[1].Namespace(), blobsWithDiffNamespaces[1].Namespace()))
			},
		},
		{
			name: "get blob with incorrect commitment",
			doFn: func() (interface{}, error) {
				b, err := service.Get(ctx, 1,
					blobsWithDiffNamespaces[0].Namespace(),
					blobsWithDiffNamespaces[1].Commitment,
				)
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
				proof, err := service.GetProof(ctx, 1,
					blobsWithDiffNamespaces[1].Namespace(),
					blobsWithDiffNamespaces[1].Commitment,
				)
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

				rawShares, err := BlobsToShares(blobsWithDiffNamespaces[1])
				require.NoError(t, err)
				verifyFn(t, rawShares, proof, blobsWithDiffNamespaces[1].Namespace())
			},
		},
		{
			name: "verify inclusion",
			doFn: func() (interface{}, error) {
				proof, err := service.GetProof(ctx, 1,
					blobsWithDiffNamespaces[0].Namespace(),
					blobsWithDiffNamespaces[0].Commitment,
				)
				require.NoError(t, err)
				return service.Included(ctx, 1,
					blobsWithDiffNamespaces[0].Namespace(),
					proof,
					blobsWithDiffNamespaces[0].Commitment,
				)
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
				proof, err := service.GetProof(ctx, 1,
					blobsWithDiffNamespaces[1].Namespace(),
					blobsWithDiffNamespaces[1].Commitment,
				)
				require.NoError(t, err)
				return service.Included(ctx, 1,
					blobsWithDiffNamespaces[0].Namespace(),
					proof,
					blobsWithDiffNamespaces[0].Commitment,
				)
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

				proof, err := service.GetProof(ctx, 1,
					blobsWithDiffNamespaces[1].Namespace(),
					blobsWithDiffNamespaces[1].Commitment,
				)
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
				proof0, err := service.GetProof(ctx, 1,
					blobsWithDiffNamespaces[0].Namespace(),
					blobsWithDiffNamespaces[0].Commitment,
				)
				if err != nil {
					return nil, err
				}
				proof1, err := service.GetProof(ctx, 1,
					blobsWithDiffNamespaces[1].Namespace(),
					blobsWithDiffNamespaces[1].Commitment,
				)
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
			name: "empty result and err when blobs were not found ",
			doFn: func() (interface{}, error) {
				nid, err := share.NewBlobNamespaceV0(tmrand.Bytes(squarens.NamespaceVersionZeroIDSize))
				require.NoError(t, err)
				return service.GetAll(ctx, 1, []share.Namespace{nid})
			},
			expectedResult: func(i interface{}, err error) {
				blobs, ok := i.([]*Blob)
				require.True(t, ok)
				assert.Nil(t, blobs)
				assert.NoError(t, err)
			},
		},
		{
			name: "marshal proof",
			doFn: func() (interface{}, error) {
				proof, err := service.GetProof(ctx, 1,
					blobsWithDiffNamespaces[1].Namespace(),
					blobsWithDiffNamespaces[1].Commitment,
				)
				require.NoError(t, err)
				return json.Marshal(proof)
			},
			expectedResult: func(i interface{}, err error) {
				require.NoError(t, err)
				jsonData, ok := i.([]byte)
				require.True(t, ok)
				var proof Proof
				require.NoError(t, json.Unmarshal(jsonData, &proof))

				newProof, err := service.GetProof(ctx, 1,
					blobsWithDiffNamespaces[1].Namespace(),
					blobsWithDiffNamespaces[1].Commitment,
				)
				require.NoError(t, err)
				require.NoError(t, proof.equal(*newProof))
			},
		},
		{
			name: "internal error",
			doFn: func() (interface{}, error) {
				ctrl := gomock.NewController(t)
				innerGetter := service.shareGetter
				getterWrapper := mock.NewMockGetter(ctrl)
				getterWrapper.EXPECT().
					GetSharesByNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(
						func(
							ctx context.Context, h *header.ExtendedHeader, ns share.Namespace,
						) (shwap.NamespaceData, error) {
							if ns.Equals(blobsWithDiffNamespaces[0].Namespace()) {
								return nil, errors.New("internal error")
							}
							return innerGetter.GetSharesByNamespace(ctx, h, ns)
						}).AnyTimes()

				service.shareGetter = getterWrapper
				return service.GetAll(ctx, 1,
					[]share.Namespace{
						blobsWithDiffNamespaces[0].Namespace(),
						blobsWithSameNamespace[0].Namespace(),
					},
				)
			},
			expectedResult: func(res interface{}, err error) {
				blobs, ok := res.([]*Blob)
				assert.True(t, ok)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "internal error")
				assert.Equal(t, blobs[0].Namespace(), blobsWithSameNamespace[0].Namespace())
				assert.NotEmpty(t, blobs)
				assert.Len(t, blobs, len(blobsWithSameNamespace))
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

	padding0, err := appshares.NamespacePaddingShare(ns1, appconsts.ShareVersionZero)
	require.NoError(t, err)
	padding1, err := appshares.NamespacePaddingShare(ns2, appconsts.ShareVersionZero)
	require.NoError(t, err)
	rawShares0, err := BlobsToShares(blobs[0])
	require.NoError(t, err)
	rawShares1, err := BlobsToShares(blobs[1])
	require.NoError(t, err)

	rawShares := make([][]byte, 0)
	rawShares = append(rawShares, append(rawShares0, padding0.ToBytes())...)
	rawShares = append(rawShares, append(rawShares1, padding1.ToBytes())...)
	service := createService(ctx, t, rawShares)

	newBlob, err := service.Get(ctx, 1, blobs[1].Namespace(), blobs[1].Commitment)
	require.NoError(t, err)
	assert.Equal(t, newBlob.Commitment, blobs[1].Commitment)

	resultShares, err := BlobsToShares(newBlob)
	require.NoError(t, err)

	h, err := service.headerGetter(ctx, 1)
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

	shares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	service := createService(ctx, t, shares)

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
		shareOffset += appshares.SparseSharesNeeded(uint32(len(blob.Data)))
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

	padding, err := appshares.NamespacePaddingShare(ns, appconsts.ShareVersionZero)
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
	service := createService(ctx, t, rawShares)

	newBlobs, err := service.GetAll(ctx, 1, []share.Namespace{blobs[0].Namespace()})
	require.NoError(t, err)
	assert.Equal(t, len(newBlobs), len(blobs))

	resultShares, err := BlobsToShares(newBlobs...)
	require.NoError(t, err)

	h, err := service.headerGetter(ctx, 1)
	require.NoError(t, err)
	shareOffset := 0
	for i, blob := range newBlobs {
		require.True(t, blobs[i].compareCommitments(blob.Commitment))

		row, col := calculateIndex(len(h.DAH.RowRoots), blob.index)
		sh, err := service.shareGetter.GetShare(ctx, h, row, col)
		require.NoError(t, err)

		assert.Equal(t, sh, resultShares[shareOffset])
		shareOffset += appshares.SparseSharesNeeded(uint32(len(blob.Data)))
	}
}

func TestAllPaddingSharesInEDS(t *testing.T) {
	nid, err := share.NewBlobNamespaceV0(tmrand.Bytes(7))
	require.NoError(t, err)
	padding, err := appshares.NamespacePaddingShare(nid.ToAppNamespace(), appconsts.ShareVersionZero)
	require.NoError(t, err)

	rawShares := make([]share.Share, 16)
	for i := 0; i < 16; i++ {
		rawShares[i] = padding.ToBytes()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	service := createService(ctx, t, rawShares)
	newBlobs, err := service.GetAll(ctx, 1, []share.Namespace{nid})
	require.NoError(t, err)
	assert.Empty(t, newBlobs)
}

func TestSkipPaddingsAndRetrieveBlob(t *testing.T) {
	nid, err := share.NewBlobNamespaceV0(tmrand.Bytes(7))
	require.NoError(t, err)
	padding, err := appshares.NamespacePaddingShare(nid.ToAppNamespace(), appconsts.ShareVersionZero)
	require.NoError(t, err)

	rawShares := make([]share.Share, 0, 64)
	for i := 0; i < 58; i++ {
		rawShares = append(rawShares, padding.ToBytes())
	}

	appBlob, err := blobtest.GenerateV0Blobs([]int{6}, true)
	require.NoError(t, err)
	appBlob[0].NamespaceVersion = uint32(nid[0])
	appBlob[0].NamespaceId = nid[1:]

	blobs, err := convertBlobs(appBlob...)
	require.NoError(t, err)
	sh, err := BlobsToShares(blobs[0])
	require.NoError(t, err)

	rawShares = append(rawShares, sh...)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	service := createService(ctx, t, rawShares)
	newBlob, err := service.GetAll(ctx, 1, []share.Namespace{nid})
	require.NoError(t, err)
	require.Len(t, newBlob, 1)
	require.True(t, newBlob[0].compareCommitments(blobs[0].Commitment))
}

func TestService_Subscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	appBlobs, err := blobtest.GenerateV0Blobs([]int{16, 16, 16}, true)
	require.NoError(t, err)
	blobs, err := convertBlobs(appBlobs...)
	require.NoError(t, err)

	service := createServiceWithSub(ctx, t, blobs)
	err = service.Start(ctx)
	require.NoError(t, err)

	t.Run("successful subscription", func(t *testing.T) {
		ns := blobs[0].Namespace()
		subCh, err := service.Subscribe(ctx, ns)
		require.NoError(t, err)

		for i := uint64(0); i < uint64(len(blobs)); i++ {
			select {
			case resp := <-subCh:
				assert.Equal(t, i+1, resp.Height)
				assert.Equal(t, blobs[i].Data, resp.Blobs[0].Data)
			case <-time.After(time.Second * 2):
				t.Fatalf("timeout waiting for subscription response %d", i)
			}
		}
	})

	t.Run("subscription with no matching blobs", func(t *testing.T) {
		ns, err := share.NewBlobNamespaceV0([]byte("nonexist"))
		require.NoError(t, err)

		subCh, err := service.Subscribe(ctx, ns)
		require.NoError(t, err)

		// check that empty responses are received (as no matching blobs were found)
		for i := uint64(0); i < uint64(len(blobs)); i++ {
			select {
			case resp := <-subCh:
				assert.Empty(t, resp.Blobs)
				assert.Equal(t, i+1, resp.Height)
			case <-time.After(time.Second * 2):
				t.Fatalf("timeout waiting for empty subscription response %d", i)
			}
		}
	})

	t.Run("subscription cancellation", func(t *testing.T) {
		ns := blobs[0].Namespace()
		subCtx, subCancel := context.WithCancel(ctx)
		subCh, err := service.Subscribe(subCtx, ns)
		require.NoError(t, err)

		// cancel the subscription context after receiving the first response
		select {
		case <-subCh:
			subCancel()
		case <-time.After(time.Second * 2):
			t.Fatal("timeout waiting for first subscription response")
		}

		select {
		case _, ok := <-subCh:
			assert.False(t, ok, "expected subscription channel to be closed")
		case <-time.After(time.Second * 2):
			t.Fatal("timeout waiting for subscription channel to close")
		}
	})

	t.Run("graceful shutdown", func(t *testing.T) {
		ns := blobs[0].Namespace()
		subCh, err := service.Subscribe(ctx, ns)
		require.NoError(t, err)

		// cancel the subscription context after receiving the last response
		for range blobs {
			select {
			case val := <-subCh:
				if val.Height == uint64(len(blobs)) {
					err = service.Stop(context.Background())
					require.NoError(t, err)
				}
			case <-time.After(time.Second * 2):
				t.Fatal("timeout waiting for subscription response")
			}
		}

		select {
		case _, ok := <-subCh:
			assert.False(t, ok, "expected subscription channel to be closed")
		case <-time.After(time.Second * 2):
			t.Fatal("timeout waiting for subscription channel to close")
		}
	})
}

func TestService_Subscribe_MultipleNamespaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	appBlobs1, err := blobtest.GenerateV0Blobs([]int{100, 100}, true)
	require.NoError(t, err)
	appBlobs2, err := blobtest.GenerateV0Blobs([]int{100, 100}, true)
	for i := range appBlobs2 {
		// if we don't do this, appBlobs1 and appBlobs2 will share a NS
		appBlobs2[i].GetNamespaceId()[len(appBlobs2[i].GetNamespaceId())-1] = 0xDE
	}
	require.NoError(t, err)

	blobs1, err := convertBlobs(appBlobs1...)
	require.NoError(t, err)
	blobs2, err := convertBlobs(appBlobs2...)
	require.NoError(t, err)

	//nolint: gocritic
	allBlobs := append(blobs1, blobs2...)

	service := createServiceWithSub(ctx, t, allBlobs)
	err = service.Start(ctx)
	require.NoError(t, err)

	ns1 := blobs1[0].Namespace()
	ns2 := blobs2[0].Namespace()

	subCh1, err := service.Subscribe(ctx, ns1)
	require.NoError(t, err)
	subCh2, err := service.Subscribe(ctx, ns2)
	require.NoError(t, err)

	for i := uint64(0); i < uint64(len(allBlobs)); i++ {
		select {
		case resp := <-subCh1:
			assert.Equal(t, i+1, resp.Height)
			if i < uint64(len(blobs1)) {
				assert.NotEmpty(t, resp.Blobs)
				for _, b := range resp.Blobs {
					assert.Equal(t, ns1, b.Namespace())
				}
			} else {
				assert.Empty(t, resp.Blobs)
			}
		case resp := <-subCh2:
			assert.Equal(t, i+1, resp.Height)
			if i >= uint64(len(blobs1)) {
				assert.NotEmpty(t, resp.Blobs)
				for _, b := range resp.Blobs {
					assert.Equal(t, ns2, b.Namespace())
				}
			} else {
				assert.Empty(t, resp.Blobs)
			}
		case <-time.After(time.Second * 2):
			t.Fatalf("timeout waiting for subscription responses %d", i)
		}
	}
}

// BenchmarkGetByCommitment-12    	    1869	    571663 ns/op	 1085371 B/op	    6414 allocs/op
func BenchmarkGetByCommitment(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	b.Cleanup(cancel)
	appBlobs, err := blobtest.GenerateV0Blobs([]int{32, 32}, true)
	require.NoError(b, err)

	blobs, err := convertBlobs(appBlobs...)
	require.NoError(b, err)

	shares, err := BlobsToShares(blobs...)
	require.NoError(b, err)
	service := createService(ctx, b, shares)
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

func createServiceWithSub(ctx context.Context, t testing.TB, blobs []*Blob) *Service {
	bs := ipld.NewMemBlockservice()
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	headerStore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)
	edsses := make([]*rsmt2d.ExtendedDataSquare, len(blobs))
	for i, blob := range blobs {
		rawShares, err := BlobsToShares(blob)
		require.NoError(t, err)
		eds, err := ipld.AddShares(ctx, rawShares, bs)
		require.NoError(t, err)
		edsses[i] = eds
	}
	headers := headertest.ExtendedHeadersFromEdsses(t, edsses)

	err = headerStore.Init(ctx, headers[0])
	require.NoError(t, err)

	err = headerStore.Append(ctx, headers[1:]...)
	require.NoError(t, err)

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return headers[height-1], nil
		// return headerStore.GetByHeight(ctx, height)
	}
	fn2 := func(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
		headerChan := make(chan *header.ExtendedHeader, len(headers))
		defer func() {
			for _, h := range headers {
				time.Sleep(time.Millisecond * 100)
				headerChan <- h
			}
		}()
		return headerChan, nil
	}
	ctrl := gomock.NewController(t)
	shareGetter := mock.NewMockGetter(ctrl)

	shareGetter.EXPECT().GetSharesByNamespace(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(ctx context.Context, h *header.ExtendedHeader, ns share.Namespace) (shwap.NamespaceData, error) {
			idx := int(h.Height()) - 1
			accessor := &eds.Rsmt2D{ExtendedDataSquare: edsses[idx]}
			nd, err := eds.NamespaceData(ctx, accessor, ns)
			return nd, err
		})
	return NewService(nil, shareGetter, fn, fn2)
}

func createService(ctx context.Context, t testing.TB, shares []share.Share) *Service {
	odsSize := int(utils.SquareSize(len(shares)))
	square, err := rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)))
	require.NoError(t, err)

	accessor := &eds.Rsmt2D{ExtendedDataSquare: square}
	ctrl := gomock.NewController(t)
	shareGetter := mock.NewMockGetter(ctrl)
	shareGetter.EXPECT().GetSharesByNamespace(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(ctx context.Context, h *header.ExtendedHeader, ns share.Namespace) (shwap.NamespaceData, error) {
			nd, err := eds.NamespaceData(ctx, accessor, ns)
			return nd, err
		})
	shareGetter.EXPECT().GetShare(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(ctx context.Context, h *header.ExtendedHeader, row, col int) (share.Share, error) {
			s, err := accessor.Sample(ctx, row, col)
			return s.Share, err
		})

	// create header and put it into the store
	h := headertest.ExtendedHeaderFromEDS(t, 1, square)
	batching := ds_sync.MutexWrap(ds.NewMapDatastore())
	headerStore, err := store.NewStore[*header.ExtendedHeader](batching)
	require.NoError(t, err)
	err = headerStore.Init(ctx, h)
	require.NoError(t, err)

	fn := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
		return headerStore.GetByHeight(ctx, height)
	}
	fn2 := func(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
		return nil, fmt.Errorf("not implemented")
	}
	return NewService(nil, shareGetter, fn, fn2)
}

// TestProveCommitmentAllCombinations tests proving all the commitments in a block.
// The number of shares per blob increases with each blob to cover proving a large number
// of possibilities.
func TestProveCommitmentAllCombinations(t *testing.T) {
	tests := map[string]struct {
		blobSize int
	}{
		"blobs that take less than a share": {blobSize: 350},
		"blobs that take 2 shares":          {blobSize: 1000},
		"blobs that take ~10 shares":        {blobSize: 5000},
		"large blobs ~100 shares":           {blobSize: 50000},
		"large blobs ~150 shares":           {blobSize: 75000},
		"large blobs ~300 shares":           {blobSize: 150000},
		"very large blobs ~1500 shares":     {blobSize: 750000},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			proveAndVerifyShareCommitments(t, tc.blobSize)
		})
	}
}

func proveAndVerifyShareCommitments(t *testing.T, blobSize int) {
	msgs, blobs, nss, eds, _, _, dataRoot := edstest.GenerateTestBlock(t, blobSize, 10)
	for msgIndex, msg := range msgs {
		t.Run(fmt.Sprintf("msgIndex=%d", msgIndex), func(t *testing.T) {
			blb, err := NewBlob(uint8(blobs[msgIndex].GetShareVersion()), nss[msgIndex].Bytes(), blobs[msgIndex].GetData())
			require.NoError(t, err)
			blobShares, err := BlobsToShares(blb)
			require.NoError(t, err)
			// compute the commitment
			actualCommitmentProof, err := ProveCommitment(eds, nss[msgIndex].Bytes(), blobShares)
			require.NoError(t, err)

			// make sure the actual commitment attests to the data
			require.NoError(t, actualCommitmentProof.Validate())
			valid, err := actualCommitmentProof.Verify(
				dataRoot,
				appconsts.DefaultSubtreeRootThreshold,
			)
			require.NoError(t, err)
			require.True(t, valid)

			// generate an expected proof and verify it's valid
			expectedCommitmentProof := generateCommitmentProofFromBlock(t, eds, nss[msgIndex].Bytes(), blobs[msgIndex], dataRoot)
			require.NoError(t, expectedCommitmentProof.Validate())
			valid, err = expectedCommitmentProof.Verify(
				dataRoot,
				appconsts.DefaultSubtreeRootThreshold,
			)
			require.NoError(t, err)
			require.True(t, valid)

			// make sure the expected proof is the same as the actual on
			assert.Equal(t, expectedCommitmentProof, *actualCommitmentProof)

			// make sure the expected commitment commits to the subtree roots in the result proof
			actualCommitment, _ := merkle.ProofsFromByteSlices(actualCommitmentProof.SubtreeRoots)
			assert.Equal(t, msg.ShareCommitments[0], actualCommitment)
		})
	}
}

// generateCommitmentProofFromBlock takes a block and a PFB index and generates the commitment proof
// using the traditional way of doing, instead of using the API.
func generateCommitmentProofFromBlock(
	t *testing.T,
	eds *rsmt2d.ExtendedDataSquare,
	ns share.Namespace,
	blob *blob.Blob,
	dataRoot []byte,
) CommitmentProof {
	// create the blob from the data
	blb, err := NewBlob(
		uint8(blob.GetShareVersion()),
		ns,
		blob.GetData(),
	)
	require.NoError(t, err)

	// convert the blob to a number of shares
	blobShares, err := BlobsToShares(blb)
	require.NoError(t, err)

	// find the first share of the blob in the ODS
	startShareIndex := -1
	for i, sh := range eds.FlattenedODS() {
		if bytes.Equal(sh, blobShares[0]) {
			startShareIndex = i
			break
		}
	}
	require.Greater(t, startShareIndex, 0)

	// create an inclusion proof of the blob using the share range instead of the commitment
	sharesProof, err := pkgproof.NewShareInclusionProofFromEDS(
		eds,
		ns.ToAppNamespace(),
		appshares.NewRange(startShareIndex, startShareIndex+len(blobShares)),
	)
	require.NoError(t, err)
	require.NoError(t, sharesProof.Validate(dataRoot))

	// calculate the subtree roots
	subtreeRoots := make([][]byte, 0)
	dataCursor := 0
	for _, proof := range sharesProof.ShareProofs {
		ranges, err := nmt.ToLeafRanges(
			int(proof.Start),
			int(proof.End),
			inclusion.SubTreeWidth(len(blobShares), appconsts.DefaultSubtreeRootThreshold),
		)
		require.NoError(t, err)
		roots, err := computeSubtreeRoots(
			blobShares[dataCursor:int32(dataCursor)+proof.End-proof.Start],
			ranges,
			int(proof.Start),
		)
		require.NoError(t, err)
		subtreeRoots = append(subtreeRoots, roots...)
		dataCursor += int(proof.End - proof.Start)
	}

	// convert the nmt proof to be accepted by the commitment proof
	nmtProofs := make([]*nmt.Proof, 0)
	for _, proof := range sharesProof.ShareProofs {
		nmtProof := nmt.NewInclusionProof(int(proof.Start), int(proof.End), proof.Nodes, true)
		nmtProofs = append(nmtProofs, &nmtProof)
	}

	commitmentProof := CommitmentProof{
		SubtreeRoots:      subtreeRoots,
		SubtreeRootProofs: nmtProofs,
		NamespaceID:       sharesProof.NamespaceId,
		RowProof:          *sharesProof.RowProof,
		NamespaceVersion:  uint8(sharesProof.NamespaceVersion),
	}

	return commitmentProof
}
