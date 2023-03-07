package share

import (
	"context"
	"math/rand"
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
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

func TestGetShare(t *testing.T) {
	const size = 8

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bServ := mdutils.Bserv()

	// generate random shares for the nmt
	shares := RandShares(t, size*size)
	eds, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	for i, leaf := range shares {
		row := i / size
		pos := i - (size * row)
		share, err := GetShare(ctx, bServ, ipld.MustCidFromNamespacedSha256(eds.RowRoots()[row]), pos, size*2)
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
	quarterShares := RandShares(t, shareCount)
	allShares := RandShares(t, shareCount)

	testCases := []struct {
		name      string
		shares    []Share
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

			eds, err := rsmt2d.ComputeExtendedDataSquare(tc.shares, rsmt2d.NewRSGF8Codec(), wrapper.NewConstructor(squareSize))
			require.NoError(t, err)

			// calculate roots using the first complete square
			rowRoots := eds.RowRoots()
			colRoots := eds.ColRoots()

			flat := ExtractEDS(eds)

			// recover a partially complete square
			rdata := removeRandShares(flat, tc.d)
			eds, err = rsmt2d.ImportExtendedDataSquare(
				rdata,
				rsmt2d.NewRSGF8Codec(),
				wrapper.NewConstructor(squareSize),
			)
			require.NoError(t, err)

			err = eds.Repair(rowRoots, colRoots)
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
				return
			}
			assert.NoError(t, err)

			reds, err := rsmt2d.ImportExtendedDataSquare(rdata, rsmt2d.NewRSGF8Codec(), wrapper.NewConstructor(squareSize))
			require.NoError(t, err)
			// check that the squares are equal
			assert.Equal(t, ExtractEDS(eds), ExtractEDS(reds))
		})
	}
}

func Test_ConvertEDStoShares(t *testing.T) {
	squareWidth := 16
	shares := RandShares(t, squareWidth*squareWidth)

	// compute extended square
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		shares,
		rsmt2d.NewRSGF8Codec(),
		wrapper.NewConstructor(uint64(squareWidth)),
	)
	require.NoError(t, err)

	resshares := ExtractODS(eds)
	require.Equal(t, shares, resshares)
}

// removes d shares from data
func removeRandShares(data [][]byte, d int) [][]byte {
	count := len(data)
	// remove shares randomly
	for i := 0; i < d; {
		ind := rand.Intn(count)
		if len(data[ind]) == 0 {
			continue
		}
		data[ind] = nil
		i++
	}
	return data
}

func TestGetSharesByNamespace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bServ := mdutils.Bserv()

	var tests = []struct {
		rawData []Share
	}{
		{rawData: RandShares(t, 4)},
		{rawData: RandShares(t, 16)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// choose random nID from rand shares
			expected := tt.rawData[len(tt.rawData)/2]
			nID := expected[:NamespaceSize]

			// change rawData to contain several shares with same nID
			tt.rawData[(len(tt.rawData)/2)+1] = expected
			// put raw data in BlockService
			eds, err := AddShares(ctx, tt.rawData, bServ)
			require.NoError(t, err)

			var shares []Share
			for _, row := range eds.RowRoots() {
				rcid := ipld.MustCidFromNamespacedSha256(row)
				rowShares, _, err := GetSharesByNamespace(ctx, bServ, rcid, nID, len(eds.RowRoots()))
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

func TestGetLeavesByNamespace_IncompleteData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bServ := mdutils.Bserv()

	shares := RandShares(t, 16)

	// set all shares to the same namespace id
	nid := shares[0][:NamespaceSize]

	for i, nspace := range shares {
		if i == len(shares) {
			break
		}

		copy(nspace[:NamespaceSize], nid)
	}

	eds, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	roots := eds.RowRoots()

	// remove the second share from the first row
	rcid := ipld.MustCidFromNamespacedSha256(roots[0])
	node, err := ipld.GetNode(ctx, bServ, rcid)
	require.NoError(t, err)

	// Left side of the tree contains the original shares
	data, err := ipld.GetNode(ctx, bServ, node.Links()[0].Cid)
	require.NoError(t, err)

	// Second share is the left side's right child
	l, err := ipld.GetNode(ctx, bServ, data.Links()[0].Cid)
	require.NoError(t, err)
	r, err := ipld.GetNode(ctx, bServ, l.Links()[1].Cid)
	require.NoError(t, err)
	err = bServ.DeleteBlock(ctx, r.Cid())
	require.NoError(t, err)

	rData := ipld.NewRetrievedData(len(shares), ipld.WithLeaves())
	err = ipld.GetLeavesByNamespace(ctx, bServ, rcid, nid, rData)
	leaves := rData.CollectLeaves()
	assert.Nil(t, leaves[1])
	assert.Equal(t, 4, len(leaves))
	require.Error(t, err)
}

func TestGetLeavesByNamespace_AbsentNamespaceId(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bServ := mdutils.Bserv()

	shares := RandShares(t, 16)

	minNid := make([]byte, NamespaceSize)
	midNid := make([]byte, NamespaceSize)
	maxNid := make([]byte, NamespaceSize)

	numberOfShares := len(shares)

	copy(minNid, shares[0][:NamespaceSize])
	copy(maxNid, shares[numberOfShares-1][:NamespaceSize])
	copy(midNid, shares[numberOfShares/2][:NamespaceSize])

	// create min nid missing data by replacing first namespace id with second
	minNidMissingData := make([]Share, len(shares))
	copy(minNidMissingData, shares)
	copy(minNidMissingData[0][:NamespaceSize], shares[1][:NamespaceSize])

	// create max nid missing data by replacing last namespace id with second last
	maxNidMissingData := make([]Share, len(shares))
	copy(maxNidMissingData, shares)
	copy(maxNidMissingData[numberOfShares-1][:NamespaceSize], shares[numberOfShares-2][:NamespaceSize])

	// create mid nid missing data by replacing middle namespace id with the one after
	midNidMissingData := make([]Share, len(shares))
	copy(midNidMissingData, shares)
	copy(midNidMissingData[numberOfShares/2][:NamespaceSize], shares[(numberOfShares/2)+1][:NamespaceSize])

	var tests = []struct {
		name       string
		data       []Share
		missingNid []byte
	}{
		{name: "Namespace id less than the minimum namespace in data", data: minNidMissingData, missingNid: minNid},
		{name: "Namespace id greater than the maximum namespace in data", data: maxNidMissingData, missingNid: maxNid},
		{name: "Namespace id in range but still missing", data: midNidMissingData, missingNid: midNid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eds, err := AddShares(ctx, shares, bServ)
			require.NoError(t, err)
			assertNoRowContainsNID(t, bServ, eds, tt.missingNid)
		})
	}
}

func TestGetLeavesByNamespace_MultipleRowsContainingSameNamespaceId(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bServ := mdutils.Bserv()

	shares := RandShares(t, 16)

	// set all shares to the same namespace and data but the last one
	nid := shares[0][:NamespaceSize]
	commonNamespaceData := shares[0]

	for i, nspace := range shares {
		if i == len(shares)-1 {
			break
		}

		copy(nspace, commonNamespaceData)
	}

	eds, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	for _, row := range eds.RowRoots() {
		rcid := ipld.MustCidFromNamespacedSha256(row)
		data := ipld.NewRetrievedData(len(shares), ipld.WithLeaves())
		err := ipld.GetLeavesByNamespace(ctx, bServ, rcid, nid, data)
		assert.Nil(t, err)
		leaves := data.CollectLeaves()
		for _, node := range leaves {
			// test that the data returned by getLeavesByNamespace for nid
			// matches the commonNamespaceData that was copied across almost all data
			assert.Equal(t, commonNamespaceData, node.RawData()[NamespaceSize:])
		}
	}
}

func TestGetSharesWithProofsByNamespace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	var tests = []struct {
		rawData []Share
	}{
		{rawData: RandShares(t, 4)},
		{rawData: RandShares(t, 16)},
		{rawData: RandShares(t, 64)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			rand.Seed(time.Now().UnixNano())
			// choose random range in shares slice
			from := rand.Intn(len(tt.rawData))
			to := rand.Intn(len(tt.rawData))

			if to < from {
				from, to = to, from
			}

			expected := tt.rawData[from]
			nID := expected[:NamespaceSize]

			// change rawData to contain several shares with same nID
			for i := from; i <= to; i++ {
				tt.rawData[i] = expected
			}

			// put raw data in BlockService
			eds, err := AddShares(ctx, tt.rawData, bServ)
			require.NoError(t, err)

			var shares []Share
			for _, row := range eds.RowRoots() {
				rcid := ipld.MustCidFromNamespacedSha256(row)
				rowShares, proof, err := GetSharesByNamespace(ctx, bServ, rcid, nID, len(eds.RowRoots()))
				require.NoError(t, err)
				if rowShares != nil {
					require.NotNil(t, proof)
					// append shares to check integrity later
					shares = append(shares, rowShares...)

					// construct nodes from shares by prepending namespace
					var leaves [][]byte
					for _, sh := range rowShares {
						leaves = append(leaves, append(sh[:NamespaceSize], sh...))
					}

					proofNodes := make([][]byte, 0, len(proof.Nodes))
					for _, n := range proof.Nodes {
						proofNodes = append(proofNodes, ipld.NamespacedSha256FromCID(n))
					}

					// construct new proof
					inclusionProof := nmt.NewInclusionProof(
						proof.Start,
						proof.End,
						proofNodes,
						ipld.NMTIgnoreMaxNamespace)

					// verify inclusion
					verified := inclusionProof.VerifyNamespace(
						sha256.New(),
						nID,
						leaves,
						ipld.NamespacedSha256FromCID(rcid))
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
		// {"64", 64}, // test case too large for CI with race detector
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(tt.origWidth))
			defer cancel()

			bs := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))

			eds := RandEDS(t, tt.origWidth)
			_, err := AddShares(ctx, ExtractODS(eds), blockservice.New(bs, offline.Exchange(bs)))
			require.NoError(t, err)

			out, err := bs.AllKeysChan(ctx)
			require.NoError(t, err)

			var count int
			for range out {
				count++
			}
			extendedWidth := tt.origWidth * 2
			assert.Equalf(t, count, ipld.BatchSize(extendedWidth), "batchSize(%v)", extendedWidth)
		})
	}
}

func assertNoRowContainsNID(
	t *testing.T,
	bServ blockservice.BlockService,
	eds *rsmt2d.ExtendedDataSquare,
	nID namespace.ID,
) {
	rowRootCount := len(eds.RowRoots())
	// get all row root cids
	rowRootCIDs := make([]cid.Cid, rowRootCount)
	for i, rowRoot := range eds.RowRoots() {
		rowRootCIDs[i] = ipld.MustCidFromNamespacedSha256(rowRoot)
	}

	// for each row root cid check if the minNID exists
	for _, rowCID := range rowRootCIDs {
		data := ipld.NewRetrievedData(rowRootCount, ipld.WithProofs())
		err := ipld.GetLeavesByNamespace(context.Background(), bServ, rowCID, nID, data)
		leaves := data.CollectLeaves()
		assert.Nil(t, leaves)
		assert.Nil(t, err)
	}
}
