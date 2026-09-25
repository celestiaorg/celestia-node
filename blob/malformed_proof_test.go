package blob

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/nmt"
)

// TestIncluded_MalformedProofDoesNotPanic feeds Included (exposed as blob.Included over
// JSON-RPC with read permission) proofs that have the right number of rows but are otherwise
// malformed. Included must report the proof as invalid instead of panicking.
func TestIncluded_MalformedProofDoesNotPanic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	libBlobs, err := libshare.GenerateV0Blobs([]int{10, 6}, false) // 16 shares -> 4x4 ODS
	require.NoError(t, err)
	blobs, err := convertBlobs(libBlobs...)
	require.NoError(t, err)
	shares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	service := createService(ctx, t, shares)

	ns, com := blobs[0].Namespace(), blobs[0].Commitment
	valid, err := service.GetProof(ctx, 1, ns, com)
	require.NoError(t, err)
	require.NotEmpty(t, (*valid)[0].Nodes())

	// same number of row proofs, but the first one is missing its last node
	truncated := make(Proof, len(*valid))
	copy(truncated, *valid)
	p0 := (*valid)[0]
	short := nmt.NewInclusionProof(p0.Start(), p0.End(), p0.Nodes()[:len(p0.Nodes())-1], true)
	truncated[0] = &short

	// same number of row proofs, all null (what `[null]` decodes to over JSON)
	var nullProof Proof
	nullJSON := "["
	for i := range *valid {
		if i > 0 {
			nullJSON += ","
		}
		nullJSON += "null"
	}
	nullJSON += "]"
	require.NoError(t, json.Unmarshal([]byte(nullJSON), &nullProof))
	require.Len(t, nullProof, len(*valid))

	for name, pr := range map[string]*Proof{"truncated nodes": &truncated, "null row proof": &nullProof} {
		t.Run(name, func(t *testing.T) {
			require.NotPanics(t, func() {
				_, err := service.Included(ctx, 1, ns, pr, com)
				require.ErrorIs(t, err, ErrInvalidProof)
			})
		})
	}
}

// TestCommitmentProofVerify_NullEntriesDoNotPanic shows that CommitmentProof.Verify, which
// clients run on proofs returned by (possibly untrusted) nodes, panics on JSON null entries
// instead of returning an error.
func TestCommitmentProofVerify_NullEntriesDoNotPanic(t *testing.T) {
	ns := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0xAB}, 10))
	b, err := NewBlob(libshare.ShareVersionZero, ns, bytes.Repeat([]byte{0x42}, 1600), nil) // 4 shares
	require.NoError(t, err)
	nodeBlob := []*Blob{b}
	blobShares, err := BlobsToShares(nodeBlob[0])
	require.NoError(t, err)
	eds, dataRoot := buildTestEDS(t, blobShares)
	cp, err := ProveCommitment(eds, nodeBlob[0].Namespace(), blobShares)
	require.NoError(t, err)
	require.NoError(t, cp.Verify(dataRoot, nodeBlob[0].Commitment))

	raw, err := json.Marshal(cp)
	require.NoError(t, err)

	cases := map[string]func(m map[string]any){
		"null subtree root proof": func(m map[string]any) {
			prfs := m["subtree_root_proofs"].([]any)
			for i := range prfs {
				prfs[i] = nil
			}
		},
		"null row proof": func(m map[string]any) {
			rp := m["row_proof"].(map[string]any)
			prfs := rp["proofs"].([]any)
			for i := range prfs {
				prfs[i] = nil
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var m map[string]any
			require.NoError(t, json.Unmarshal(raw, &m))
			mutate(m)
			mutated, err := json.Marshal(m)
			require.NoError(t, err)

			var bad CommitmentProof
			require.NoError(t, json.Unmarshal(mutated, &bad))
			require.NotPanics(t, func() {
				require.Error(t, bad.Verify(dataRoot, nodeBlob[0].Commitment))
			})
		})
	}
}
