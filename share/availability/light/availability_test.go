package light

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	mrand "math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	appshares "github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
)

func init() {
	// randomize quadrant fetching, otherwise quadrant sampling is deterministic
	rand.Seed(time.Now().UnixNano())
}

func TestSharesAvailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getter, dah := GetterWithRandSquare(t, 16)
	avail := TestAvailability(getter)
	err := avail.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}

func TestSharesAvailableFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getter, _ := GetterWithRandSquare(t, 16)
	avail := TestAvailability(getter)
	empty := header.EmptyDAH()
	err := avail.SharesAvailable(ctx, &empty)
	assert.Error(t, err)
}

func TestShareAvailableOverMocknet_Light(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := availability_test.NewTestDAGNet(ctx, t)
	_, root := RandNode(net, 16)
	nd := Node(net)
	net.ConnectAll()

	err := nd.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}

func TestGetShare(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	getter, dah := GetterWithRandSquare(t, n)

	for i := range make([]bool, n) {
		for j := range make([]bool, n) {
			sh, err := getter.GetShare(ctx, dah, i, j)
			assert.NotNil(t, sh)
			assert.NoError(t, err)
		}
	}
}

func TestService_GetSharesByNamespace(t *testing.T) {
	var tests = []struct {
		squareSize         int
		expectedShareCount int
	}{
		{squareSize: 4, expectedShareCount: 2},
		{squareSize: 16, expectedShareCount: 2},
		{squareSize: 128, expectedShareCount: 2},
	}

	for _, tt := range tests {
		t.Run("size: "+strconv.Itoa(tt.squareSize), func(t *testing.T) {
			getter, bServ := EmptyGetter()
			n := tt.squareSize * tt.squareSize
			randShares := share.RandShares(t, n)
			idx1 := (n - 1) / 2
			idx2 := n / 2
			if tt.expectedShareCount > 1 {
				// make it so that two rows have the same namespace ID
				copy(randShares[idx2][:8], randShares[idx1][:8])
			}
			root := availability_test.FillBS(t, bServ, randShares)
			randNID := randShares[idx1][:8]

			shares, err := getter.GetSharesByNamespace(context.Background(), root, randNID)
			require.NoError(t, err)
			require.NoError(t, shares.Verify(root, randNID))
			flattened := shares.Flatten()
			assert.Len(t, flattened, tt.expectedShareCount)
			for _, value := range flattened {
				assert.Equal(t, randNID, []byte(share.ID(value)))
			}
			if tt.expectedShareCount > 1 {
				// idx1 is always smaller than idx2
				assert.Equal(t, randShares[idx1], flattened[0])
				assert.Equal(t, randShares[idx2], flattened[1])
			}
		})
	}
}

func TestGetShares(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	getter, dah := GetterWithRandSquare(t, n)

	eds, err := getter.GetEDS(ctx, dah)
	require.NoError(t, err)
	gotDAH := da.NewDataAvailabilityHeader(eds)

	require.True(t, dah.Equals(&gotDAH))
}

func TestService_GetSharesByNamespaceNotFound(t *testing.T) {
	getter, root := GetterWithRandSquare(t, 1)
	root.RowsRoots = nil

	shares, err := getter.GetSharesByNamespace(context.Background(), root, []byte{1, 1, 1, 1, 1, 1, 1, 1})
	assert.Len(t, shares, 0)
	assert.NoError(t, err)
}

func BenchmarkService_GetSharesByNamespace(b *testing.B) {
	var tests = []struct {
		amountShares int
	}{
		{amountShares: 4},
		{amountShares: 16},
		{amountShares: 128},
	}

	for _, tt := range tests {
		b.Run(strconv.Itoa(tt.amountShares), func(b *testing.B) {
			t := &testing.T{}
			getter, root := GetterWithRandSquare(t, tt.amountShares)
			randNID := root.RowsRoots[(len(root.RowsRoots)-1)/2][:8]
			root.RowsRoots[(len(root.RowsRoots) / 2)] = root.RowsRoots[(len(root.RowsRoots)-1)/2]
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := getter.GetSharesByNamespace(context.Background(), root, randNID)
				require.NoError(t, err)
			}
		})
	}
}

func TestSharesRoundTrip(t *testing.T) {
	getter, store := EmptyGetter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var pb tmproto.Block
	err := json.Unmarshal([]byte(sampleBlock), &pb)
	require.NoError(t, err)

	b, err := core.BlockFromProto(&pb)
	require.NoError(t, err)

	namespace, err := hex.DecodeString("00001337BEEF0000")
	require.NoError(t, err)
	namespaceBefore, err := hex.DecodeString("0000000000000123")
	require.NoError(t, err)
	namespaceAfter, err := hex.DecodeString("1234000000000123")
	require.NoError(t, err)

	type testCase struct {
		name       string
		messages   [][]byte
		namespaces [][]byte
	}

	cases := []testCase{
		{
			"original test case",
			[][]byte{b.Data.Blobs[0].Data},
			[][]byte{namespace}},
		{
			"one short message",
			[][]byte{{1, 2, 3, 4}},
			[][]byte{namespace}},
		{
			"one short before other namespace",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}},
			[][]byte{namespace, namespaceAfter},
		},
		{
			"one short after other namespace",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}},
			[][]byte{namespaceBefore, namespace},
		},
		{
			"two short messages",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}},
			[][]byte{namespace, namespace},
		},
		{
			"two short messages before other namespace",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}, {7, 8, 9}},
			[][]byte{namespace, namespace, namespaceAfter},
		},
		{
			"two short messages after other namespace",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}, {7, 8, 9}},
			[][]byte{namespaceBefore, namespace, namespace},
		},
	}
	randBytes := func(n int) []byte {
		bs := make([]byte, n)
		mrand.Read(bs)
		return bs
	}
	for i := 128; i < 4192; i += mrand.Intn(256) {
		l := strconv.Itoa(i)
		cases = append(cases, testCase{
			"one " + l + " bytes message",
			[][]byte{randBytes(i)},
			[][]byte{namespace},
		})
		cases = append(cases, testCase{
			"one " + l + " bytes before other namespace",
			[][]byte{randBytes(i), randBytes(1 + mrand.Intn(i))},
			[][]byte{namespace, namespaceAfter},
		})
		cases = append(cases, testCase{
			"one " + l + " bytes after other namespace",
			[][]byte{randBytes(1 + mrand.Intn(i)), randBytes(i)},
			[][]byte{namespaceBefore, namespace},
		})
		cases = append(cases, testCase{
			"two " + l + " bytes messages",
			[][]byte{randBytes(i), randBytes(i)},
			[][]byte{namespace, namespace},
		})
		cases = append(cases, testCase{
			"two " + l + " bytes messages before other namespace",
			[][]byte{randBytes(i), randBytes(i), randBytes(1 + mrand.Intn(i))},
			[][]byte{namespace, namespace, namespaceAfter},
		})
		cases = append(cases, testCase{
			"two " + l + " bytes messages after other namespace",
			[][]byte{randBytes(1 + mrand.Intn(i)), randBytes(i), randBytes(i)},
			[][]byte{namespaceBefore, namespace, namespace},
		})
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// prepare data
			b.Data.Blobs = make([]core.Blob, len(tc.messages))
			b.SquareSize = 16
			var msgsInNamespace [][]byte
			require.Equal(t, len(tc.namespaces), len(tc.messages))
			for i := range tc.messages {
				b.Data.Blobs[i] = core.Blob{NamespaceID: tc.namespaces[i], Data: tc.messages[i]}
				if bytes.Equal(tc.namespaces[i], namespace) {
					msgsInNamespace = append(msgsInNamespace, tc.messages[i])
				}
			}

			// TODO: set useShareIndexes to true. This requires updating the
			// transaction data in this test to include share indexes.
			shares, err := appshares.Split(b.Data, false)
			if err != nil {
				t.Fatal(err)
			}

			// test round trip using only encoding, without IPLD
			{
				myShares := make([][]byte, 0)
				for _, sh := range shares {
					if bytes.Equal(namespace, sh[:8]) {
						myShares = append(myShares, sh)
					}
				}
				blobs, err := appshares.ParseBlobs(myShares)
				require.NoError(t, err)
				assert.Len(t, blobs, len(msgsInNamespace))
				for i := range blobs {
					assert.Equal(t, msgsInNamespace[i], blobs[i].Data)
				}
			}

			// test full round trip - with IPLD + decoding shares
			{
				extSquare, err := share.AddShares(ctx, appshares.ToBytes(shares), store)
				require.NoError(t, err)

				dah := da.NewDataAvailabilityHeader(extSquare)
				shares, err := getter.GetSharesByNamespace(ctx, &dah, namespace)
				require.NoError(t, err)
				require.NoError(t, shares.Verify(&dah, namespace))
				require.NotEmpty(t, shares)

				blobs, err := appshares.ParseBlobs(shares.Flatten())
				require.NoError(t, err)
				assert.Len(t, blobs, len(msgsInNamespace))
				for i := range blobs {
					assert.Equal(t, namespace, []byte(blobs[i].NamespaceID))
					assert.Equal(t, msgsInNamespace[i], blobs[i].Data)
				}
			}
		})
	}
}

// this is a sample block
//
//go:embed "testdata/sample-block.json"
var sampleBlock string
