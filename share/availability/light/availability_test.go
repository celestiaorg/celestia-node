package light

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	mrand "math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/namespace"
	appshares "github.com/celestiaorg/celestia-app/pkg/shares"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
)

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
			totalShares := tt.squareSize * tt.squareSize
			randShares := share.RandShares(t, totalShares)
			idx1 := (totalShares - 1) / 2
			idx2 := totalShares / 2
			if tt.expectedShareCount > 1 {
				// make it so that two rows have the same namespace ID
				copy(randShares[idx2][:namespace.NamespaceSize], randShares[idx1][:namespace.NamespaceSize])
			}
			root := availability_test.FillBS(t, bServ, randShares)
			randNID := randShares[idx1][:namespace.NamespaceSize]

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
		t.Run("last two rows of a 4x4 square that have the same namespace ID have valid NMT proofs", func(t *testing.T) {
			squareSize := 4
			totalShares := squareSize * squareSize
			getter, bServ := EmptyGetter()
			randShares := share.RandShares(t, totalShares)
			lastNID := randShares[totalShares-1][:namespace.NamespaceSize]
			for i := totalShares / 2; i < totalShares; i++ {
				copy(randShares[i][:namespace.NamespaceSize], lastNID)
			}
			root := availability_test.FillBS(t, bServ, randShares)

			shares, err := getter.GetSharesByNamespace(context.Background(), root, lastNID)
			require.NoError(t, err)
			require.NoError(t, shares.Verify(root, lastNID))
		})
		t.Run("no shares are returned when GetSharesByNamespace is invoked with random namespace ID", func(t *testing.T) {
			squareSize := 4
			totalShares := squareSize * squareSize
			getter, bServ := EmptyGetter()
			randShares := share.RandShares(t, totalShares)
			root := availability_test.FillBS(t, bServ, randShares)
			randomNID := namespace.RandomBlobNamespace()

			shares, err := getter.GetSharesByNamespace(context.Background(), root, randomNID.Bytes())
			require.NoError(t, err)
			require.Equal(t, 0, len(shares.Flatten()))
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

	shares, err := getter.GetSharesByNamespace(context.Background(), root, namespace.RandomNamespace().Bytes())
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

	namespaceA := namespace.MustNewV0(bytes.Repeat([]byte{0xa}, namespace.NamespaceVersionZeroIDSize))
	namespaceB := namespace.MustNewV0(bytes.Repeat([]byte{0xb}, namespace.NamespaceVersionZeroIDSize))
	namespaceC := namespace.MustNewV0(bytes.Repeat([]byte{0xc}, namespace.NamespaceVersionZeroIDSize))

	type testCase struct {
		name       string
		messages   [][]byte
		namespaces []namespace.Namespace
	}

	cases := []testCase{
		{
			"original test case",
			[][]byte{b.Data.Blobs[0].Data},
			[]namespace.Namespace{namespaceB}},
		{
			"one short message",
			[][]byte{{1, 2, 3, 4}},
			[]namespace.Namespace{namespaceB}},
		{
			"one short before other namespace",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}},
			[]namespace.Namespace{namespaceB, namespaceC},
		},
		{
			"one short after other namespace",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}},
			[]namespace.Namespace{namespaceA, namespaceB},
		},
		{
			"two short messages",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}},
			[]namespace.Namespace{namespaceB, namespaceB},
		},
		{
			"two short messages before other namespace",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}, {7, 8, 9}},
			[]namespace.Namespace{namespaceB, namespaceB, namespaceC},
		},
		{
			"two short messages after other namespace",
			[][]byte{{1, 2, 3, 4}, {4, 5, 6, 7}, {7, 8, 9}},
			[]namespace.Namespace{namespaceA, namespaceB, namespaceB},
		},
	}
	randBytes := func(n int) []byte {
		bs := make([]byte, n)
		_, _ = rand.Read(bs)
		return bs
	}
	for i := 128; i < 4192; i += mrand.Intn(256) {
		l := strconv.Itoa(i)
		cases = append(cases, testCase{
			"one " + l + " bytes message",
			[][]byte{randBytes(i)},
			[]namespace.Namespace{namespaceB},
		})
		cases = append(cases, testCase{
			"one " + l + " bytes before other namespace",
			[][]byte{randBytes(i), randBytes(1 + mrand.Intn(i))},
			[]namespace.Namespace{namespaceB, namespaceC},
		})
		cases = append(cases, testCase{
			"one " + l + " bytes after other namespace",
			[][]byte{randBytes(1 + mrand.Intn(i)), randBytes(i)},
			[]namespace.Namespace{namespaceA, namespaceB},
		})
		cases = append(cases, testCase{
			"two " + l + " bytes messages",
			[][]byte{randBytes(i), randBytes(i)},
			[]namespace.Namespace{namespaceB, namespaceB},
		})
		cases = append(cases, testCase{
			"two " + l + " bytes messages before other namespace",
			[][]byte{randBytes(i), randBytes(i), randBytes(1 + mrand.Intn(i))},
			[]namespace.Namespace{namespaceB, namespaceB, namespaceC},
		})
		cases = append(cases, testCase{
			"two " + l + " bytes messages after other namespace",
			[][]byte{randBytes(1 + mrand.Intn(i)), randBytes(i), randBytes(i)},
			[]namespace.Namespace{namespaceA, namespaceB, namespaceB},
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
				b.Data.Blobs[i] = core.Blob{
					NamespaceID:      tc.namespaces[i].ID,
					Data:             tc.messages[i],
					NamespaceVersion: tc.namespaces[i].Version,
				}
				if bytes.Equal(tc.namespaces[i].Bytes(), namespaceB.Bytes()) {
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
					ns, err := sh.Namespace()
					require.NoError(t, err)
					if bytes.Equal(namespaceB.Bytes(), ns.Bytes()) {
						myShares = append(myShares, sh.ToBytes())
					}
				}
				appShares, err := appshares.FromBytes(myShares)
				assert.NoError(t, err)
				blobs, err := appshares.ParseBlobs(appShares)
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
				shares, err := getter.GetSharesByNamespace(ctx, &dah, namespaceB.Bytes())
				require.NoError(t, err)
				require.NoError(t, shares.Verify(&dah, namespaceB.Bytes()))
				require.NotEmpty(t, shares)

				appShares, err := appshares.FromBytes(shares.Flatten())
				assert.NoError(t, err)
				blobs, err := appshares.ParseBlobs(appShares)
				require.NoError(t, err)
				assert.Len(t, blobs, len(msgsInNamespace))
				for i := range blobs {
					assert.Equal(t, namespaceB.ID, blobs[i].NamespaceID)
					assert.Equal(t, namespaceB.Version, blobs[i].NamespaceVersion)
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
