package share

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"math"
	mrand "math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/ipld"
)

func TestGetShare(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	serv, dah := RandLightServiceWithSquare(t, n)
	err := serv.Start(ctx)
	require.NoError(t, err)

	for i := range make([]bool, n) {
		for j := range make([]bool, n) {
			share, err := serv.GetShare(ctx, dah, i, j)
			assert.NotNil(t, share)
			assert.NoError(t, err)
		}
	}

	err = serv.Stop(ctx)
	require.NoError(t, err)
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
			serv, dag := RandLightService()
			n := tt.squareSize * tt.squareSize
			randShares := RandShares(t, n)
			idx1 := (n - 1) / 2
			idx2 := n / 2
			if tt.expectedShareCount > 1 {
				// make it so that two rows have the same namespace ID
				copy(randShares[idx2][:8], randShares[idx1][:8])
			}
			root := FillDag(t, dag, randShares)
			randNID := randShares[idx1][:8]

			shares, err := serv.GetSharesByNamespace(context.Background(), root, randNID)
			require.NoError(t, err)
			assert.Len(t, shares, tt.expectedShareCount)
			for _, value := range shares {
				assert.Equal(t, randNID, []byte(GetID(value)))
			}
			if tt.expectedShareCount > 1 {
				// idx1 is always smaller than idx2
				assert.Equal(t, randShares[idx1], shares[0])
				assert.Equal(t, randShares[idx2], shares[1])
			}
		})
	}
}

func TestGetShares(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	serv, dah := RandLightServiceWithSquare(t, n)
	err := serv.Start(ctx)
	require.NoError(t, err)

	shares, err := serv.GetShares(ctx, dah)
	require.NoError(t, err)

	flattened := make([][]byte, 0, len(shares)*2)
	for _, row := range shares {
		flattened = append(flattened, row...)
	}
	// generate DAH from shares returned by `GetShares` to compare
	// calculated DAH to expected DAH
	squareSize := uint64(math.Sqrt(float64(len(flattened))))
	eds, err := da.ExtendShares(squareSize, flattened)
	require.NoError(t, err)
	gotDAH := da.NewDataAvailabilityHeader(eds)

	require.True(t, dah.Equals(&gotDAH))

	err = serv.Stop(ctx)
	require.NoError(t, err)
}

func TestService_GetSharesByNamespaceNotFound(t *testing.T) {
	serv, root := RandLightServiceWithSquare(t, 1)
	root.RowsRoots = nil

	shares, err := serv.GetSharesByNamespace(context.Background(), root, []byte(""))
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
			serv, root := RandLightServiceWithSquare(t, tt.amountShares)
			randNID := root.RowsRoots[(len(root.RowsRoots)-1)/2][:8]
			root.RowsRoots[(len(root.RowsRoots) / 2)] = root.RowsRoots[(len(root.RowsRoots)-1)/2]
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := serv.GetSharesByNamespace(context.Background(), root, randNID)
				require.NoError(t, err)
			}
		})
	}
}

func TestSharesRoundTrip(t *testing.T) {
	serv, store := RandLightService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var b core.Block
	err := json.Unmarshal([]byte(sampleBlock), &b)
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
			[][]byte{b.Data.Messages.MessagesList[0].Data},
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
		bytes := make([]byte, n)
		mrand.Read(bytes)
		return bytes
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
			b.Data.Messages.MessagesList = make([]core.Message, len(tc.messages))
			var msgsInNamespace [][]byte
			require.Equal(t, len(tc.namespaces), len(tc.messages))
			for i := range tc.messages {
				b.Data.Messages.MessagesList[i] = core.Message{NamespaceID: tc.namespaces[i], Data: tc.messages[i]}
				if bytes.Equal(tc.namespaces[i], namespace) {
					msgsInNamespace = append(msgsInNamespace, tc.messages[i])
				}
			}

			namespacedShares, _ := b.Data.ComputeShares()

			// test round trip using only encoding, without IPLD
			{
				myShares := make([][]byte, 0)
				for _, sh := range namespacedShares.RawShares() {
					if bytes.Equal(namespace, sh[:8]) {
						myShares = append(myShares, sh)
					}
				}
				msgs, err := core.ParseMsgs(myShares)
				require.NoError(t, err)
				assert.Len(t, msgs.MessagesList, len(msgsInNamespace))
				for i := range msgs.MessagesList {
					assert.Equal(t, msgsInNamespace[i], msgs.MessagesList[i].Data)
				}
			}

			// test full round trip - with IPLD + decoding shares
			{
				extSquare, err := ipld.AddShares(ctx, namespacedShares.RawShares(), store)
				require.NoError(t, err)

				dah := da.NewDataAvailabilityHeader(extSquare)
				shares, err := serv.GetSharesByNamespace(ctx, &dah, namespace)
				require.NoError(t, err)
				require.NotEmpty(t, shares)

				msgs, err := core.ParseMsgs(shares)
				require.NoError(t, err)
				assert.Len(t, msgs.MessagesList, len(msgsInNamespace))
				for i := range msgs.MessagesList {
					assert.Equal(t, namespace, []byte(msgs.MessagesList[i].NamespaceID))
					assert.Equal(t, msgsInNamespace[i], msgs.MessagesList[i].Data)
				}
			}
		})
	}
}

// this is a sample block from devnet-2 which originally showed the issue with share ordering
//go:embed "testdata/block-825320.json"
var sampleBlock string
