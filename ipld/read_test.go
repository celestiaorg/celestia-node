package ipld

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math"
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
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

func TestGetLeafData(t *testing.T) {
	const leaves = 16

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	dag := mdutils.Mock()

	// generate random shares for the nmt
	shares := RandNamespacedShares(t, leaves)

	// create a random tree
	root, err := getNmtRoot(ctx, dag, shares.Raw())
	require.NoError(t, err)

	for i, leaf := range shares {
		data, err := GetLeafData(ctx, root, uint32(i), uint32(len(shares)), dag)
		require.NoError(t, err)
		assert.True(t, bytes.Equal(leaf.Share, data))
	}
}

func TestBlockRecovery(t *testing.T) {
	originalSquareWidth := 8
	shareCount := originalSquareWidth * originalSquareWidth
	extendedSquareWidth := 2 * originalSquareWidth
	extendedShareCount := extendedSquareWidth * extendedSquareWidth

	// generate test data
	quarterShares := RandNamespacedShares(t, shareCount)
	allShares := RandNamespacedShares(t, shareCount)

	testCases := []struct {
		name      string
		shares    NamespacedShares
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
			squareSize := uint64(math.Sqrt(float64(len(tc.shares))))

			// create trees for creating roots
			tree := wrapper.NewErasuredNamespacedMerkleTree(squareSize)
			recoverTree := wrapper.NewErasuredNamespacedMerkleTree(squareSize)

			eds, err := rsmt2d.ComputeExtendedDataSquare(tc.shares.Raw(), rsmt2d.NewRSGF8Codec(), tree.Constructor)
			require.NoError(t, err)

			// calculate roots using the first complete square
			rowRoots := eds.RowRoots()
			colRoots := eds.ColRoots()

			flat := flatten(eds)

			// recover a partially complete square
			reds, err := rsmt2d.RepairExtendedDataSquare(
				rowRoots,
				colRoots,
				removeRandShares(flat, tc.d),
				rsmt2d.NewRSGF8Codec(),
				recoverTree.Constructor,
			)

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
				return
			}
			assert.NoError(t, err)

			// check that the squares are equal
			assert.Equal(t, flatten(eds), flatten(reds))
		})
	}
}

func TestRetrieveBlockData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dag := mdutils.Mock()

	type test struct {
		name       string
		squareSize int
	}
	tests := []test{
		{"1x1(min)", 1},
		{"32x32(med)", 32},
		{"128x128(max)", MaxSquareSize},
	}
	for _, tc := range tests {
		if tc.squareSize == MaxSquareSize {
			t.Skip("skipping as it spawns too many goroutines")
		}
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// generate EDS
			eds := generateRandEDS(t, tc.squareSize)

			shares := ExtractODSShares(eds)

			in, err := PutData(ctx, shares, dag)
			require.NoError(t, err)

			// limit with deadline, specifically retrieval
			ctx, cancel := context.WithTimeout(ctx, time.Second*2)
			defer cancel()

			dah := da.NewDataAvailabilityHeader(in)

			out, err := RetrieveData(ctx, &dah, dag, rsmt2d.NewRSGF8Codec())
			require.NoError(t, err)
			assert.True(t, EqualEDS(in, out))
		})
	}
}

func Test_ConvertEDStoShares(t *testing.T) {
	squareWidth := 16
	origShares := RandNamespacedShares(t, squareWidth*squareWidth)
	rawshares := origShares.Raw()

	// create the nmt wrapper to generate row and col commitments
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(squareWidth))

	// compute extended square
	eds, err := rsmt2d.ComputeExtendedDataSquare(rawshares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
	require.NoError(t, err)

	resshares := ExtractODSShares(eds)
	require.Equal(t, rawshares, resshares)
}

func generateRandEDS(t *testing.T, originalSquareWidth int) *rsmt2d.ExtendedDataSquare {
	shareCount := originalSquareWidth * originalSquareWidth

	// generate test data
	nsshares := RandNamespacedShares(t, shareCount)

	shares := nsshares.Raw()

	// create the nmt wrapper to generate row and col commitments
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(originalSquareWidth))

	// compute extended square
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
	require.NoError(t, err)
	return eds
}

func flatten(eds *rsmt2d.ExtendedDataSquare) [][]byte {
	flattenedEDSSize := eds.Width() * eds.Width()
	out := make([][]byte, flattenedEDSSize)
	count := 0
	for i := uint(0); i < eds.Width(); i++ {
		for _, share := range eds.Row(i) {
			out[count] = share
			count++
		}
	}
	return out
}

// getNmtRoot generates the nmt root of some namespaced data
func getNmtRoot(
	ctx context.Context,
	dag format.NodeAdder,
	namespacedData [][]byte,
) (cid.Cid, error) {
	na := NewNmtNodeAdder(ctx, dag)
	tree := nmt.New(sha256.New(), nmt.NamespaceIDSize(NamespaceSize), nmt.NodeVisitor(na.Visit))
	for _, leaf := range namespacedData {
		err := tree.Push(leaf)
		if err != nil {
			return cid.Undef, err
		}
	}

	// call Root early as it initiates saving
	root := tree.Root()
	if err := na.Commit(); err != nil {
		return cid.Undef, err
	}

	return plugin.CidFromNamespacedSha256(root)
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

func TestGetLeavesByNamespace(t *testing.T) {
	var tests = []struct {
		rawData [][]byte
	}{
		{rawData: generateRandNamespacedRawData(16, NamespaceSize, plugin.ShareSize)},
		{rawData: generateRandNamespacedRawData(16, NamespaceSize, 8)},
		{rawData: generateRandNamespacedRawData(4, NamespaceSize, plugin.ShareSize)},
		{rawData: generateRandNamespacedRawData(16, NamespaceSize, 8)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			squareSize := uint64(math.Sqrt(float64(len(tt.rawData))))

			// choose random nID from rand shares
			expected := tt.rawData[len(tt.rawData)/2]
			nID := expected[:NamespaceSize]

			// change rawData to contain several shares with same nID
			tt.rawData[(len(tt.rawData)/2)+1] = expected

			// generate DAH
			eds, err := da.ExtendShares(squareSize, tt.rawData)
			require.NoError(t, err)
			dah := da.NewDataAvailabilityHeader(eds)

			// put raw data in DAG
			dag := mdutils.Mock()
			_, err = PutData(context.Background(), tt.rawData, dag)
			require.NoError(t, err)

			rowRootCIDs, err := rowRootsByNamespaceID(nID, &dah)
			require.NoError(t, err)

			for _, rowCID := range rowRootCIDs {
				nodes, err := GetLeavesByNamespace(context.Background(), dag, rowCID, nID)
				require.NoError(t, err)

				for _, node := range nodes {
					// TODO @renaynay: nID is prepended twice for some reason.
					share := node.RawData()[1:]
					assert.Equal(t, expected, share[NamespaceSize:])
				}
			}
		})
	}
}

func TestGetLeavesByNamespace_AbsentNamespaceId(t *testing.T) {
	rawData := RandNamespacedShares(t, 16).Raw()

	minNid := make([]byte, NamespaceSize)
	midNid := make([]byte, NamespaceSize)
	maxNid := make([]byte, NamespaceSize)

	numberOfShares := len(rawData)

	copy(minNid, rawData[0][:NamespaceSize])
	copy(maxNid, rawData[numberOfShares-1][:NamespaceSize])
	copy(midNid, rawData[numberOfShares/2][:NamespaceSize])

	// create min nid missing data by replacing first namespace id with second
	minNidMissingData := make([][]byte, len(rawData))
	copy(minNidMissingData, rawData)
	copy(minNidMissingData[0][:NamespaceSize], rawData[1][:NamespaceSize])

	// create max nid missing data by replacing last namespace id with second last
	maxNidMissingData := make([][]byte, len(rawData))
	copy(maxNidMissingData, rawData)
	copy(maxNidMissingData[numberOfShares-1][:NamespaceSize], rawData[numberOfShares-2][:NamespaceSize])

	// create mid nid missing data by replacing middle namespace id with the one after
	midNidMissingData := make([][]byte, len(rawData))
	copy(midNidMissingData, rawData)
	copy(midNidMissingData[numberOfShares/2][:NamespaceSize], rawData[(numberOfShares/2)+1][:NamespaceSize])

	var tests = []struct {
		name       string
		data       [][]byte
		missingNid []byte
	}{
		{name: "Namespace id less than the minimum namespace in data", data: minNidMissingData, missingNid: minNid},
		{name: "Namespace id greater than the maximum namespace in data", data: maxNidMissingData, missingNid: maxNid},
		{name: "Namespace id in range but still missing", data: midNidMissingData, missingNid: midNid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag, dah := putErasuredDataToDag(t, tt.data)

			assertNoRowContainsNID(t, dag, dah, tt.missingNid)
		})
	}
}

func TestGetLeavesByNamespace_MultipleRowsContainingSameNamespaceId(t *testing.T) {
	t.Run("Same namespace id across multiple rows", func(t *testing.T) {
		rawData := RandNamespacedShares(t, 16).Raw()

		// set all shares to the same namespace and data but the last one
		nid := rawData[0][:NamespaceSize]
		commonNamespaceData := rawData[0]

		for i, nspace := range rawData {
			if i == len(rawData)-1 {
				break
			}

			copy(nspace, commonNamespaceData)
		}

		dag, dah := putErasuredDataToDag(t, rawData)

		rowRootCids, err := rowRootsByNamespaceID(nid, &dah)
		require.NoError(t, err)

		for _, rowRootCid := range rowRootCids {
			nodes, err := GetLeavesByNamespace(context.Background(), dag, rowRootCid, nid)
			require.NoError(t, err)

			for _, node := range nodes {
				// test that the data returned by GetLeavesByNamespace for nid
				// matches the commonNamespaceData that was copied across almost all data
				share := node.RawData()[1:]
				assert.Equal(t, commonNamespaceData, share[NamespaceSize:])
			}
		}
	})

}

func TestBatchSize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	bs := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	dag := merkledag.NewDAGService(blockservice.New(bs, offline.Exchange(bs)))
	origWidth := 32
	eds := generateRandEDS(t, origWidth)
	_, err := PutData(ctx, ExtractODSShares(eds), dag)
	require.NoError(t, err)

	out, err := bs.AllKeysChan(ctx)
	require.NoError(t, err)

	var count int
	for range out {
		count++
	}
	extendedWidth := origWidth * 2
	assert.Equal(t, count, batchSize(extendedWidth))
}

func putErasuredDataToDag(t *testing.T, rawData [][]byte) (format.DAGService, da.DataAvailabilityHeader) {
	// calc square size
	squareSize := uint64(math.Sqrt(float64(len(rawData))))

	// generate DAH
	eds, err := da.ExtendShares(squareSize, rawData)
	require.NoError(t, err)
	dah := da.NewDataAvailabilityHeader(eds)

	// put raw data in DAG
	dag := mdutils.Mock()
	_, err = PutData(context.Background(), rawData, dag)
	require.NoError(t, err)

	return dag, dah
}

func assertNoRowContainsNID(
	t *testing.T,
	dag format.DAGService,
	dah da.DataAvailabilityHeader,
	nID namespace.ID,
) {
	// get all row root cids
	rowRootCIDs := make([]cid.Cid, len(dah.RowsRoots))
	for i, rowRoot := range dah.RowsRoots {
		rowRootCIDs[i] = plugin.MustCidFromNamespacedSha256(rowRoot)
	}

	// for each row root cid check if the minNID exists
	for _, rowCID := range rowRootCIDs {
		_, err := GetLeavesByNamespace(context.Background(), dag, rowCID, nID)
		assert.Equal(t, ErrNotFoundInRange, err)
	}
}

// rowRootsByNamespaceID is a convenience method that finds the row root(s)
// that contain the given namespace ID.
func rowRootsByNamespaceID(nID namespace.ID, dah *da.DataAvailabilityHeader) ([]cid.Cid, error) {
	roots := make([]cid.Cid, 0)
	for _, row := range dah.RowsRoots {
		// if nID exists within range of min -> max of row, return the row
		if !nID.Less(nmt.MinNamespace(row, nID.Size())) && nID.LessOrEqual(nmt.MaxNamespace(row, nID.Size())) {
			roots = append(roots, plugin.MustCidFromNamespacedSha256(row))
		}
	}
	if len(roots) == 0 {
		return nil, ErrNotFoundInRange
	}
	return roots, nil
}

// generateRandNamespacedRawData returns random namespaced raw data for testing purposes.
func generateRandNamespacedRawData(total, nidSize, leafSize uint32) [][]byte {
	data := make([][]byte, total)
	for i := uint32(0); i < total; i++ {
		nid := make([]byte, nidSize)

		rand.Read(nid)
		data[i] = nid
	}
	sortByteArrays(data)
	for i := uint32(0); i < total; i++ {
		d := make([]byte, leafSize)

		rand.Read(d)
		data[i] = append(data[i], d...)
	}

	return data
}
