package ipld

import (
	"context"
	"errors"
	mrand "math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v5/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestGetShare(t *testing.T) {
	const size = 8

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bServ := NewMemBlockservice()

	// generate random shares for the nmt
	shares, err := libshare.RandShares(size * size)
	require.NoError(t, err)
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
	quarterShares, err := libshare.RandShares(shareCount)
	require.NoError(t, err)
	allShares, err := libshare.RandShares(shareCount)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		shares    []libshare.Share
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
		t.Run(tc.name, func(t *testing.T) {
			squareSize := utils.SquareSize(len(tc.shares))

			testEds, err := rsmt2d.ComputeExtendedDataSquare(
				libshare.ToBytes(tc.shares),
				share.DefaultRSMT2DCodec(),
				wrapper.NewConstructor(squareSize),
			)
			require.NoError(t, err)

			// calculate roots using the first complete square
			rowRoots, err := testEds.RowRoots()
			require.NoError(t, err)
			colRoots, err := testEds.ColRoots()
			require.NoError(t, err)

			flat := testEds.Flattened()

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
			assert.Equal(t, testEds.Flattened(), reds.Flattened())
		})
	}
}

func Test_ConvertEDStoShares(t *testing.T) {
	squareWidth := 16
	shares, err := libshare.RandShares(squareWidth * squareWidth)
	require.NoError(t, err)

	// compute extended square
	testEds, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(shares),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(squareWidth)),
	)
	require.NoError(t, err)

	resshares := testEds.FlattenedODS()
	require.Equal(t, libshare.ToBytes(shares), resshares)
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
	bServ := NewMemBlockservice()
	sh0, err := libshare.RandShares(4)
	require.NoError(t, err)

	sh1, err := libshare.RandShares(4)
	require.NoError(t, err)

	tests := []struct {
		rawData []libshare.Share
	}{
		{rawData: sh0},
		{rawData: sh1},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// choose random namespace from rand shares
			expected := tt.rawData[len(tt.rawData)/2]
			namespace := expected.Namespace()

			// change rawData to contain several shares with same namespace
			tt.rawData[(len(tt.rawData)/2)+1] = expected
			// put raw data in BlockService
			eds, err := AddShares(ctx, tt.rawData, bServ)
			require.NoError(t, err)

			var shares []libshare.Share
			rowRoots, err := eds.RowRoots()
			require.NoError(t, err)
			for _, row := range rowRoots {
				rowShares, _, err := GetSharesByNamespace(ctx, bServ, row, namespace, len(rowRoots))
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
	bServ := NewMemBlockservice()

	shares, err := libshare.RandShares(16)
	require.NoError(t, err)

	// set all shares to the same namespace id
	namespace := shares[0].Namespace()
	for _, shr := range shares {
		copy(shr.Namespace().Bytes(), namespace.Bytes())
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
	bServ := NewMemBlockservice()

	shares, err := libshare.RandShares(1024)
	require.NoError(t, err)
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
			copy(shr.Namespace().Bytes(), minIncluded.Bytes())
			continue
		}
		copy(shr.Namespace().Bytes(), maxIncluded.Bytes())
	}

	tests := []struct {
		name             string
		data             []libshare.Share
		missingNamespace libshare.Namespace
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
	bServ := NewMemBlockservice()

	shares, err := libshare.RandShares(16)
	require.NoError(t, err)
	// set all shares to the same namespace and data but the last one
	namespace := shares[0].Namespace()
	commonNamespaceData := shares[0]

	for i, nspace := range shares {
		if i == len(shares)-1 {
			break
		}

		copy(nspace.ToBytes(), commonNamespaceData.ToBytes())
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
			sh, err := libshare.NewShare(node.RawData()[libshare.NamespaceSize:])
			require.NoError(t, err)
			assert.Equal(t, commonNamespaceData, *sh)
		}
	}
}

func TestGetSharesWithProofsByNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	bServ := NewMemBlockservice()

	sh0, err := libshare.RandShares(4)
	require.NoError(t, err)
	sh1, err := libshare.RandShares(16)
	require.NoError(t, err)
	sh2, err := libshare.RandShares(64)
	require.NoError(t, err)
	tests := []struct {
		rawData []libshare.Share
	}{
		{rawData: sh0},
		{rawData: sh1},
		{rawData: sh2},
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
			namespace := expected.Namespace()

			// change rawData to contain several shares with same namespace
			for i := from; i <= to; i++ {
				tt.rawData[i] = expected
			}

			// put raw data in BlockService
			eds, err := AddShares(ctx, tt.rawData, bServ)
			require.NoError(t, err)

			var shares []libshare.Share
			rowRoots, err := eds.RowRoots()
			require.NoError(t, err)
			for _, row := range rowRoots {
				rowShares, proof, err := GetSharesByNamespace(ctx, bServ, row, namespace, len(rowRoots))
				outside, outsideErr := share.IsOutsideRange(namespace, row, row)
				require.NoError(t, outsideErr)
				if outside {
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
						leaves = append(leaves, append(shr.Namespace().Bytes(), shr.ToBytes()...))
					}

					// verify namespace
					verified := proof.VerifyNamespace(
						share.NewSHA256Hasher(),
						namespace.Bytes(),
						leaves,
						row)
					require.True(t, verified)

					// verify inclusion
					verified = proof.VerifyInclusion(
						share.NewSHA256Hasher(),
						namespace.Bytes(),
						libshare.ToBytes(rowShares),
						row)
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

			bs := NewMemBlockservice()

			randEds := edstest.RandEDS(t, tt.origWidth)

			shrs, err := libshare.FromBytes(randEds.FlattenedODS())
			require.NoError(t, err)
			_, err = AddShares(ctx, shrs, bs)
			require.NoError(t, err)

			out, err := bs.Blockstore().AllKeysChan(ctx)
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
	namespace libshare.Namespace,
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
		outsideRange, err := share.IsOutsideRange(namespace, rowRoot, rowRoot)
		require.NoError(t, err)
		if !outsideRange {
			// namespace does belong to namespace range of the row
			absentCount++
		}
		data := NewNamespaceData(rowRootCount, namespace, WithProofs())
		rootCID := MustCidFromNamespacedSha256(rowRoot)
		err = data.CollectLeavesByNamespace(ctx, bServ, rootCID)
		if outsideRange {
			require.ErrorIs(t, err, ErrNamespaceOutsideRange)
			continue
		}
		require.NoError(t, err)

		// if no error returned, check absence proof
		foundAbsenceRows++
		verified := data.Proof().VerifyNamespace(share.NewSHA256Hasher(), namespace.Bytes(), nil, rowRoot)
		require.True(t, verified)
	}

	if isAbsent {
		require.Equal(t, foundAbsenceRows, absentCount)
		// there should be max 1 row that has namespace range containing namespace
		require.LessOrEqual(t, absentCount, 1)
	}
}

func randomNamespaces(total int) ([]libshare.Namespace, error) {
	namespaces := make([]libshare.Namespace, total)
	for i := range namespaces {
		namespaces[i] = libshare.RandomNamespace()
	}
	sort.Slice(namespaces, func(i, j int) bool { return namespaces[i].IsLessThan(namespaces[j]) })
	return namespaces, nil
}
