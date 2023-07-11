package ipld

import (
	"bytes"
	"context"
	"errors"
	mrand "math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/minio/sha256-simd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestGetShare(t *testing.T) {
	const size = 8

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bServ := mdutils.Bserv()

	// generate random shares for the nmt
	shares := sharetest.RandShares(t, size*size)
	eds, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	for i, leaf := range shares {
		row := i / size
		pos := i - (size * row)
		rowRoots, err := eds.RowRoots()
		require.NoError(t, err)
		share, err := GetShare(ctx, bServ, MustCidFromNamespacedSha256(rowRoots[row]), pos, size*2)
		require.NoError(t, err)
		assert.Equal(t, leaf, share)
	}
}

func TestBlockRecovery(t *testing.T) {
	originalSquareWidth := 8
	shareCount := originalSquareWidth * originalSquareWidth
	extendedSquareWidth := 2 * originalSquareWidth
	extendedShareCount := extendedSquareWidth * extendedSquareWidth

	// generate test data
	quarterShares := sharetest.RandShares(t, shareCount)
	allShares := sharetest.RandShares(t, shareCount)

	testCases := []struct {
		name      string
		shares    []share.Share
		expectErr bool
		errString string
		d         int // number of shares to delete
	}{
		{"missing 1/2 shares", quarterShares, false, "", extendedShareCount / 2},
		{"missing 1/4 shares", quarterShares, false, "", extendedShareCount / 4},
		{"max missing data", quarterShares, false, "", (originalSquareWidth + 1) * (originalSquareWidth + 1)},
		{"missing all but one shares", allShares, true, "failed to solve data square", extendedShareCount - 1},
	}
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			squareSize := utils.SquareSize(len(tc.shares))

			testEds, err := rsmt2d.ComputeExtendedDataSquare(
				tc.shares,
				share.DefaultRSMT2DCodec(),
				wrapper.NewConstructor(squareSize),
			)
			require.NoError(t, err)

			// calculate roots using the first complete square
			rowRoots, err := testEds.RowRoots()
			require.NoError(t, err)
			colRoots, err := testEds.ColRoots()
			require.NoError(t, err)

			flat := share.ExtractEDS(testEds)

			// recover a partially complete square
			rdata := removeRandShares(flat, tc.d)
			testEds, err = rsmt2d.ImportExtendedDataSquare(
				rdata,
				share.DefaultRSMT2DCodec(),
				wrapper.NewConstructor(squareSize),
			)
			require.NoError(t, err)

			err = testEds.Repair(rowRoots, colRoots)
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
				return
			}
			assert.NoError(t, err)

			reds, err := rsmt2d.ImportExtendedDataSquare(rdata, share.DefaultRSMT2DCodec(), wrapper.NewConstructor(squareSize))
			require.NoError(t, err)
			// check that the squares are equal
			assert.Equal(t, share.ExtractEDS(testEds), share.ExtractEDS(reds))
		})
	}
}

func Test_ConvertEDStoShares(t *testing.T) {
	squareWidth := 16
	shares := sharetest.RandShares(t, squareWidth*squareWidth)

	// compute extended square
	testEds, err := rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(squareWidth)),
	)
	require.NoError(t, err)

	resshares := share.ExtractODS(testEds)
	require.Equal(t, shares, resshares)
}

// removes d shares from data
func removeRandShares(data [][]byte, d int) [][]byte {
	count := len(data)
	// remove shares randomly
	for i := 0; i < d; {
		ind := mrand.Intn(count)
		if len(data[ind]) == 0 {
			continue
		}
		data[ind] = nil
		i++
	}
	return data
}

func TestGetSharesByNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	var tests = []struct {
		rawData []share.Share
	}{
		{rawData: sharetest.RandShares(t, 4)},
		{rawData: sharetest.RandShares(t, 16)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// choose random namespace from rand shares
			expected := tt.rawData[len(tt.rawData)/2]
			namespace := share.GetNamespace(expected)

			// change rawData to contain several shares with same namespace
			tt.rawData[(len(tt.rawData)/2)+1] = expected
			// put raw data in BlockService
			eds, err := AddShares(ctx, tt.rawData, bServ)
			require.NoError(t, err)

			var shares []share.Share
			rowRoots, err := eds.RowRoots()
			require.NoError(t, err)
			for _, row := range rowRoots {
				rcid := MustCidFromNamespacedSha256(row)
				rowShares, _, err := GetSharesByNamespace(ctx, bServ, rcid, namespace, len(rowRoots))
				if errors.Is(err, ErrNamespaceOutsideRange) {
					continue
				}
				require.NoError(t, err)

				shares = append(shares, rowShares...)
			}

			assert.Equal(t, 2, len(shares))
			for _, share := range shares {
				assert.Equal(t, expected, share)
			}
		})
	}
}

func TestCollectLeavesByNamespace_IncompleteData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	shares := sharetest.RandShares(t, 16)

	// set all shares to the same namespace id
	namespace := share.GetNamespace(shares[0])
	for _, shr := range shares {
		copy(share.GetNamespace(shr), namespace)
	}

	eds, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	roots, err := eds.RowRoots()
	require.NoError(t, err)

	// remove the second share from the first row
	rcid := MustCidFromNamespacedSha256(roots[0])
	node, err := GetNode(ctx, bServ, rcid)
	require.NoError(t, err)

	// Left side of the tree contains the original shares
	data, err := GetNode(ctx, bServ, node.Links()[0].Cid)
	require.NoError(t, err)

	// Second share is the left side's right child
	l, err := GetNode(ctx, bServ, data.Links()[0].Cid)
	require.NoError(t, err)
	r, err := GetNode(ctx, bServ, l.Links()[1].Cid)
	require.NoError(t, err)
	err = bServ.DeleteBlock(ctx, r.Cid())
	require.NoError(t, err)

	namespaceData := NewNamespaceData(len(shares), namespace, WithLeaves())
	err = namespaceData.CollectLeavesByNamespace(ctx, bServ, rcid)
	require.Error(t, err)
	leaves := namespaceData.Leaves()
	assert.Nil(t, leaves[1])
	assert.Equal(t, 4, len(leaves))
}

func TestCollectLeavesByNamespace_AbsentNamespaceId(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	shares := sharetest.RandShares(t, 1024)

	// set all shares to the same namespace
	namespaces, err := randomNamespaces(5)
	require.NoError(t, err)
	minNamespace := namespaces[0]
	minIncluded := namespaces[1]
	midNamespace := namespaces[2]
	maxIncluded := namespaces[3]
	maxNamespace := namespaces[4]

	secondNamespaceFrom := mrand.Intn(len(shares)-2) + 1
	for i, shr := range shares {
		if i < secondNamespaceFrom {
			copy(share.GetNamespace(shr), minIncluded)
			continue
		}
		copy(share.GetNamespace(shr), maxIncluded)
	}

	var tests = []struct {
		name             string
		data             []share.Share
		missingNamespace share.Namespace
		isAbsence        bool
	}{
		{name: "Namespace less than the minimum namespace in data", data: shares, missingNamespace: minNamespace},
		{name: "Namespace greater than the maximum namespace in data", data: shares, missingNamespace: maxNamespace},
		{name: "Namespace in range but still missing", data: shares, missingNamespace: midNamespace, isAbsence: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eds, err := AddShares(ctx, shares, bServ)
			require.NoError(t, err)
			assertNoRowContainsNID(ctx, t, bServ, eds, tt.missingNamespace, tt.isAbsence)
		})
	}
}

func TestCollectLeavesByNamespace_MultipleRowsContainingSameNamespaceId(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	shares := sharetest.RandShares(t, 16)

	// set all shares to the same namespace and data but the last one
	namespace := share.GetNamespace(shares[0])
	commonNamespaceData := shares[0]

	for i, nspace := range shares {
		if i == len(shares)-1 {
			break
		}

		copy(nspace, commonNamespaceData)
	}

	eds, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	rowRoots, err := eds.RowRoots()
	require.NoError(t, err)

	for _, row := range rowRoots {
		rcid := MustCidFromNamespacedSha256(row)
		data := NewNamespaceData(len(shares), namespace, WithLeaves())
		err := data.CollectLeavesByNamespace(ctx, bServ, rcid)
		if errors.Is(err, ErrNamespaceOutsideRange) {
			continue
		}
		assert.Nil(t, err)
		leaves := data.Leaves()
		for _, node := range leaves {
			// test that the data returned by collectLeavesByNamespace for nid
			// matches the commonNamespaceData that was copied across almost all data
			assert.Equal(t, commonNamespaceData, share.GetData(node.RawData()))
		}
	}
}

func TestGetSharesWithProofsByNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	var tests = []struct {
		rawData []share.Share
	}{
		{rawData: sharetest.RandShares(t, 4)},
		{rawData: sharetest.RandShares(t, 16)},
		{rawData: sharetest.RandShares(t, 64)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			rand := mrand.New(mrand.NewSource(time.Now().UnixNano()))
			// choose random range in shares slice
			from := rand.Intn(len(tt.rawData))
			to := rand.Intn(len(tt.rawData))

			if to < from {
				from, to = to, from
			}

			expected := tt.rawData[from]
			namespace := share.GetNamespace(expected)

			// change rawData to contain several shares with same namespace
			for i := from; i <= to; i++ {
				tt.rawData[i] = expected
			}

			// put raw data in BlockService
			eds, err := AddShares(ctx, tt.rawData, bServ)
			require.NoError(t, err)

			var shares []share.Share
			rowRoots, err := eds.RowRoots()
			require.NoError(t, err)
			for _, row := range rowRoots {
				rcid := MustCidFromNamespacedSha256(row)
				rowShares, proof, err := GetSharesByNamespace(ctx, bServ, rcid, namespace, len(rowRoots))
				if namespace.IsOutsideRange(row, row) {
					require.ErrorIs(t, err, ErrNamespaceOutsideRange)
					continue
				}
				require.NoError(t, err)
				if len(rowShares) > 0 {
					require.NotNil(t, proof)
					// append shares to check integrity later
					shares = append(shares, rowShares...)

					// construct nodes from shares by prepending namespace
					var leaves [][]byte
					for _, shr := range rowShares {
						leaves = append(leaves, append(share.GetNamespace(shr), shr...))
					}

					// verify namespace
					verified := proof.VerifyNamespace(
						sha256.New(),
						namespace.ToNMT(),
						leaves,
						NamespacedSha256FromCID(rcid))
					require.True(t, verified)

					// verify inclusion
					verified = proof.VerifyInclusion(
						sha256.New(),
						namespace.ToNMT(),
						rowShares,
						NamespacedSha256FromCID(rcid))
					require.True(t, verified)
				}
			}

			// validate shares
			assert.Equal(t, to-from+1, len(shares))
			for _, share := range shares {
				assert.Equal(t, expected, share)
			}
		})
	}
}

func TestBatchSize(t *testing.T) {
	tests := []struct {
		name      string
		origWidth int
	}{
		{"2", 2},
		{"4", 4},
		{"8", 8},
		{"16", 16},
		{"32", 32},
		{"64", 64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(tt.origWidth))
			defer cancel()

			bs := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))

			randEds := edstest.RandEDS(t, tt.origWidth)
			_, err := AddShares(ctx, share.ExtractODS(randEds), blockservice.New(bs, offline.Exchange(bs)))
			require.NoError(t, err)

			out, err := bs.AllKeysChan(ctx)
			require.NoError(t, err)

			var count int
			for range out {
				count++
			}
			extendedWidth := tt.origWidth * 2
			assert.Equalf(t, count, BatchSize(extendedWidth), "batchSize(%v)", extendedWidth)
		})
	}
}

func assertNoRowContainsNID(
	ctx context.Context,
	t *testing.T,
	bServ blockservice.BlockService,
	eds *rsmt2d.ExtendedDataSquare,
	namespace share.Namespace,
	isAbsent bool,
) {
	rowRoots, err := eds.RowRoots()
	require.NoError(t, err)
	rowRootCount := len(rowRoots)
	// get all row root cids
	rowRootCIDs := make([]cid.Cid, rowRootCount)
	for i, rowRoot := range rowRoots {
		rowRootCIDs[i] = MustCidFromNamespacedSha256(rowRoot)
	}

	// for each row root cid check if the min namespace exists
	var absentCount, foundAbsenceRows int
	for _, rowRoot := range rowRoots {
		var outsideRange bool
		if !namespace.IsOutsideRange(rowRoot, rowRoot) {
			// namespace does belong to namespace range of the row
			absentCount++
		} else {
			outsideRange = true
		}
		data := NewNamespaceData(rowRootCount, namespace, WithProofs())
		rootCID := MustCidFromNamespacedSha256(rowRoot)
		err := data.CollectLeavesByNamespace(ctx, bServ, rootCID)
		if outsideRange {
			require.ErrorIs(t, err, ErrNamespaceOutsideRange)
			continue
		}
		require.NoError(t, err)

		// if no error returned, check absence proof
		foundAbsenceRows++
		verified := data.Proof().VerifyNamespace(sha256.New(), namespace.ToNMT(), nil, rowRoot)
		require.True(t, verified)
	}

	if isAbsent {
		require.Equal(t, foundAbsenceRows, absentCount)
		// there should be max 1 row that has namespace range containing namespace
		require.LessOrEqual(t, absentCount, 1)
	}
}

func randomNamespaces(total int) ([]share.Namespace, error) {
	namespaces := make([]share.Namespace, total)
	for i := range namespaces {
		namespaces[i] = sharetest.RandV0Namespace()
	}
	sort.Slice(namespaces, func(i, j int) bool { return bytes.Compare(namespaces[i], namespaces[j]) < 0 })
	return namespaces, nil
}
