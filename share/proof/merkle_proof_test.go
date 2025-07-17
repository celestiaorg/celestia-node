package proof

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func Test_VerifyMerkleProof(t *testing.T) {
	odsSize := 64
	square := edstest.RandEDS(t, odsSize)
	roots, err := share.NewAxisRoots(square)
	require.NoError(t, err)
	items := append(roots.RowRoots, roots.ColumnRoots...) //nolint: gocritic
	hash := roots.Hash()

	for from := 0; from < odsSize; from++ {
		for to := odsSize; to > from; to-- {
			proof, err := NewProof(items, int64(from), int64(to))
			require.NoError(t, err)
			isValid := proof.Verify(hash, items[from:to])
			require.True(t, isValid)
		}
	}
}

func Test_VerifyMerkleProofFailed(t *testing.T) {
	square := edstest.RandEDS(t, 64)
	roots, err := share.NewAxisRoots(square)
	require.NoError(t, err)
	items := append(roots.RowRoots, roots.ColumnRoots...) //nolint: gocritic
	hash := roots.Hash()

	invalidSquare := edstest.RandEDS(t, 64)
	invalidRoots, err := share.NewAxisRoots(invalidSquare)
	require.NoError(t, err)
	invalidItems := append(invalidRoots.RowRoots, invalidRoots.ColumnRoots...) //nolint: gocritic
	invalidHash := invalidRoots.Hash()

	from, to := 0, 0
	for {
		from = rand.Intn(len(items) / 4)
		to = rand.Intn(len(items) / 4)
		if from == to {
			continue
		}
		if from > to {
			from, to = to, from
			break
		}
	}

	proof, err := NewProof(items, int64(from), int64(to))
	require.NoError(t, err)
	isValid := proof.Verify(hash, items[from:to])
	require.True(t, isValid)

	testCases := []struct {
		name string
		doFn func(t *testing.T)
	}{
		{
			name: "empty root hash",
			doFn: func(t *testing.T) {
				isValid := proof.Verify(nil, items[from:to])
				require.False(t, isValid)
			},
		},
		{
			name: "invalid root hash",
			doFn: func(t *testing.T) {
				isValid := proof.Verify(invalidHash, items[from:to])
				require.False(t, isValid)
			},
		},
		{
			name: "empty items",
			doFn: func(t *testing.T) {
				isValid := proof.Verify(hash, nil)
				require.False(t, isValid)
			},
		},
		{
			name: "invalid items",
			doFn: func(t *testing.T) {
				isValid := proof.Verify(hash, items[from+1:to])
				require.False(t, isValid)
			},
		},
		{
			name: "mismatched items and hash",
			doFn: func(t *testing.T) {
				isValid := proof.Verify(invalidHash, invalidItems[from:to])
				require.False(t, isValid)
			},
		},
		{
			name: "correct indexes for ods",
			doFn: func(t *testing.T) {
				_, err = NewProof(items, 20, 36)
				require.NoError(t, err)
			},
		},
		{
			name: "end index is less than start index",
			doFn: func(t *testing.T) {
				_, err = NewProof(nil, 20, 0)
				require.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.doFn(t)
		})
	}
}

func Test_Encoding_Decoding_Proof(t *testing.T) {
	square := edstest.RandEDS(t, 64)
	roots, err := share.NewAxisRoots(square)
	require.NoError(t, err)

	proofNodes := [][]byte{bytes.Repeat([]byte{1}, 10), bytes.Repeat([]byte{2}, 10)}
	nmtProof1 := nmt.NewInclusionProof(0, 2, proofNodes, true)
	nmtProof2 := nmt.NewInclusionProof(5, 10, proofNodes, true)

	items := append(roots.RowRoots, roots.ColumnRoots...) //nolint: gocritic
	from, to := 0, 0
	for {
		from = rand.Intn(len(items) / 4)
		to = rand.Intn(len(items) / 4)
		if from == to {
			continue
		}
		if from > to {
			from, to = to, from
			break
		}
	}

	testCases := []struct {
		name string
		doFn func(t *testing.T)
	}{
		{
			name: "json marshaling/unmarshalling",
			doFn: func(t *testing.T) {
				dtProof, err := NewDataRootProof(&nmtProof1, &nmtProof2, roots, int64(from), int64(to))
				require.NoError(t, err)
				bb, err := json.Marshal(dtProof)
				require.NoError(t, err)
				newDtProof := &DataRootProof{}
				err = json.Unmarshal(bb, newDtProof)
				require.NoError(t, err)
				require.Equal(t, *dtProof, *newDtProof)
			},
		},
		{
			name: "json marshaling/unmarshalling with empty shares proof",
			doFn: func(t *testing.T) {
				dtProof, err := NewDataRootProof(nil, nil, roots, int64(from), int64(to))
				require.NoError(t, err)
				bb, err := json.Marshal(dtProof)
				require.NoError(t, err)
				newDtProof := &DataRootProof{}
				err = json.Unmarshal(bb, newDtProof)
				require.NoError(t, err)
				require.Equal(t, *dtProof, *newDtProof)
			},
		},
		{
			name: "json marshaling/unmarshalling full range",
			doFn: func(t *testing.T) {
				from, to = 0, 64
				dtProof, err := NewDataRootProof(nil, nil, roots, int64(from), int64(to))
				require.NoError(t, err)
				bb, err := json.Marshal(dtProof)
				require.NoError(t, err)
				newDtProof := &DataRootProof{}
				err = json.Unmarshal(bb, newDtProof)
				require.NoError(t, err)
				require.Equal(t, dtProof, newDtProof)
			},
		},
		{
			name: "proto encoding/decoding",
			doFn: func(t *testing.T) {
				dtProof, err := NewDataRootProof(&nmtProof1, &nmtProof2, roots, int64(from), int64(to))
				require.NoError(t, err)
				protoProof := dtProof.ToProto()
				require.NoError(t, err)
				newDtProof, err := DataRootProofFromProto(protoProof)
				require.NoError(t, err)
				require.Equal(t, *dtProof, *newDtProof)
			},
		},
		{
			name: "proto encoding/decoding with empty shares proof",
			doFn: func(t *testing.T) {
				dtProof, err := NewDataRootProof(nil, nil, roots, int64(from), int64(to))
				require.NoError(t, err)
				dtProof.merkleProof = nil
				protoProof := dtProof.ToProto()
				require.NoError(t, err)
				newDtProof, err := DataRootProofFromProto(protoProof)
				require.NoError(t, err)
				require.Equal(t, *dtProof, *newDtProof)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.doFn(t)
		})
	}
}
