package core

import (
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v6/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v6/pkg/da"
	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/share"
)

// TestTrulyEmptySquare ensures that a truly empty square (square size 1 and no
// txs) will be recognized as empty and return nil from `extendBlock` so that
// we do not redundantly store empty EDSes.
func TestTrulyEmptySquare(t *testing.T) {
	data := types.Data{
		Txs:        []types.Tx{},
		SquareSize: 1,
	}

	eds, err := da.ConstructEDS(data.Txs.ToSliceOfBytes(), appconsts.Version, -1)
	require.NoError(t, err)
	require.True(t, eds.Equals(share.EmptyEDS()))
}

// TestEmptySquareWithZeroTxs tests that the datahash of a block with no transactions
// is equal to the datahash of an empty eds, even if SquareSize is set to
// something non-zero. Technically, this block data is invalid because the
// construction of the square is deterministic, and the rules which dictate the
// square size do not allow for empty block data. However, should that ever
// occur, we need to ensure that the correct data root is generated.
func TestEmptySquareWithZeroTxs(t *testing.T) {
	data := types.Data{
		Txs: []types.Tx{},
	}

	eds, err := da.ConstructEDS(data.Txs.ToSliceOfBytes(), appconsts.Version, -1)
	require.NoError(t, err)
	require.True(t, eds.Equals(share.EmptyEDS()))

	// create empty shares and extend them manually
	emptyShares := libshare.TailPaddingShares(libshare.MinShareCount)
	rawEmptyShares := libshare.ToBytes(emptyShares)

	// extend the empty shares
	manualEds, err := da.ExtendShares(rawEmptyShares)
	require.NoError(t, err)

	// verify the manually extended EDS equals the empty EDS
	require.True(t, manualEds.Equals(share.EmptyEDS()))

	// verify the roots hash matches the empty EDS roots hash
	manualRoots, err := share.NewAxisRoots(manualEds)
	require.NoError(t, err)
	require.Equal(t, share.EmptyEDSRoots().Hash(), manualRoots.Hash())
}

//go:embed testdata/test_block_1035.json
var testBlock1035Data []byte

//go:embed testdata/test_block_2800000.json
var testBlock2800000Data []byte

//go:embed testdata/test_block_4000000.json
var testBlock4000000Data []byte

//go:embed testdata/test_block_6681000.json
var testBlock6681000Data []byte

//go:embed testdata/test_block_6683000.json
var testBlock6683000Data []byte

//go:embed testdata/test_block_6685000.json
var testBlock6685000Data []byte

//go:embed testdata/test_block_6700000.json
var testBlock6700000Data []byte

//go:embed testdata/test_block_8016389.json
var testBlock8016389Data []byte

//go:embed testdata/test_block_8016405.json
var testBlock8016405Data []byte

//go:embed testdata/test_block_8016382.json
var testBlock8016382Data []byte

//go:embed testdata/test_block_8016386.json
var testBlock8016386Data []byte

func TestExtendBlock_MainnetBlocks(t *testing.T) {
	type testBlockData struct {
		BlockHeight int64    `json:"block_height"`
		AppVersion  uint32   `json:"app_version"`
		Txs         []string `json:"txs"`
		SquareSize  uint64   `json:"square_size"`
		MainNetDah  string   `json:"main_net_dah"`
	}

	testCases := []struct {
		name     string
		testData []byte
	}{
		{
			name:     "Block_1035_AppV1",
			testData: testBlock1035Data,
		},
		{
			name:     "Block_2800000_AppV2",
			testData: testBlock2800000Data,
		},
		{
			name:     "Block_4000000_AppV3",
			testData: testBlock4000000Data,
		},
		{
			name:     "Block_6681000_AppV4_Size4",
			testData: testBlock6681000Data,
		},
		{
			name:     "Block_6683000_AppV4_Size32",
			testData: testBlock6683000Data,
		},
		{
			name:     "Block_6685000_AppV4_Size32",
			testData: testBlock6685000Data,
		},
		{
			name:     "Block_6700000_AppV4_Size64",
			testData: testBlock6700000Data,
		},
		{
			name:     "Block_8016389_AppV5_Size4",
			testData: testBlock8016389Data,
		},
		{
			name:     "Block_8016405_AppV5_Size16",
			testData: testBlock8016405Data,
		},
		{
			name:     "Block_8016382_AppV5_Size32",
			testData: testBlock8016382Data,
		},
		{
			name:     "Block_8016386_AppV5_Size64",
			testData: testBlock8016386Data,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var blockData testBlockData
			err := json.Unmarshal(tc.testData, &blockData)
			require.NoError(t, err)

			allTxs := make(types.Txs, len(blockData.Txs))
			for i, txBase64 := range blockData.Txs {
				tx, err := base64.StdEncoding.DecodeString(txBase64)
				require.NoError(t, err, "Failed to decode transaction %d", i)
				allTxs[i] = tx
			}

			eds, err := da.ConstructEDS(allTxs.ToSliceOfBytes(), uint64(blockData.AppVersion), -1)
			require.NoError(t, err)

			roots, err := share.NewAxisRoots(eds)
			require.NoError(t, err)
			dah := strings.ToUpper(hex.EncodeToString(roots.Hash()))
			require.NotEmpty(t, dah, "DAH should not be empty")
			require.Equal(t, blockData.MainNetDah, dah)
		})
	}
}
