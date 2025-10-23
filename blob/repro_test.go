package blob

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v6/pkg/proof"
	"github.com/celestiaorg/go-square/v3/inclusion"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/pb"
)

func TestCommitmentProofRowProofVerifyWithEmptyRoot(t *testing.T) {
	libBlob, err := libshare.GenerateV0Blobs([]int{8}, true)
	require.NoError(t, err)
	nodeBlob, err := ToNodeBlobs(libBlob...)
	require.NoError(t, err)
	roots, err := inclusion.GenerateSubtreeRoots(libBlob[0], subtreeRootThreshold)
	require.NoError(t, err)

	cp := &CommitmentProof{
		SubtreeRoots: roots,
		RowProof: proof.RowProof{
			Proofs: []*proof.Proof{{}},
		},
	}
	root := []byte{0xd3, 0x4d, 0x34}
	if err := cp.Verify(root, nodeBlob[0].Commitment); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

func TestCommitmentProofRowProofVerify(t *testing.T) {
	libBlob, err := libshare.GenerateV0Blobs([]int{8}, true)
	require.NoError(t, err)
	nodeBlob, err := ToNodeBlobs(libBlob...)
	require.NoError(t, err)
	roots, err := inclusion.GenerateSubtreeRoots(libBlob[0], subtreeRootThreshold)
	require.NoError(t, err)
	cp := &CommitmentProof{
		SubtreeRoots: roots,
		RowProof: proof.RowProof{
			Proofs: []*proof.Proof{{}},
		},
	}
	if err := cp.Verify(nil, nodeBlob[0].Commitment); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

func TestCommitmentProofVerifySliceBound(t *testing.T) {
	libBlob, err := libshare.GenerateV0Blobs([]int{8}, true)
	require.NoError(t, err)
	nodeBlob, err := ToNodeBlobs(libBlob...)
	require.NoError(t, err)
	roots, err := inclusion.GenerateSubtreeRoots(libBlob[0], subtreeRootThreshold)
	require.NoError(t, err)

	proof := nmt.ProtoToProof(pb.Proof{End: 1})
	cp := &CommitmentProof{
		SubtreeRoots: roots,
		SubtreeRootProofs: []*nmt.Proof{
			&proof,
		},
	}
	if err := cp.Verify([]byte{0xd3, 0x4d, 0x34}, nodeBlob[0].Commitment); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

func TestBlobUnmarshalRepro(t *testing.T) {
	blob := new(Blob)
	if err := blob.UnmarshalJSON([]byte("{}")); err == nil {
		t.Fatal("expected a non-nil error")
	}
}
