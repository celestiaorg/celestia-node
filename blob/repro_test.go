package blob_test

import (
	"testing"

	"github.com/celestiaorg/celestia-app/v2/pkg/proof"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/pb"
)

// Reported at https://github.com/celestiaorg/celestia-node/issues/3731.
func TestCommitmentProofRowProofVerifyWithEmptyRoot(t *testing.T) {
	cp := &blob.CommitmentProof{
		RowProof: proof.RowProof{
			Proofs: []*proof.Proof{{}},
		},
	}
	root := []byte{0xd3, 0x4d, 0x34}
	if _, err := cp.Verify(root, 1); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

// Reported at https://github.com/celestiaorg/celestia-node/issues/3730.
func TestCommitmentProofRowProofVerify(t *testing.T) {
	cp := &blob.CommitmentProof{
		RowProof: proof.RowProof{
			Proofs: []*proof.Proof{{}},
		},
	}
	if _, err := cp.Verify(nil, 1); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

// Reported at https://github.com/celestiaorg/celestia-node/issues/3729.
func TestCommitmentProofVerifySliceBound(t *testing.T) {
	proof := nmt.ProtoToProof(pb.Proof{End: 1})
	cp := &blob.CommitmentProof{
		SubtreeRootProofs: []*nmt.Proof{
			&proof,
		},
	}
	if _, err := cp.Verify(nil, 1); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

// Reported at https://github.com/celestiaorg/celestia-node/issues/3728.
func TestCommitmentProofVerifyZeroSubThreshold(t *testing.T) {
	cp := new(blob.CommitmentProof)
	if _, err := cp.Verify(nil, 0); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

// Reported at https://github.com/celestiaorg/celestia-node/issues/3727.
func TestBlobUnmarshalRepro(t *testing.T) {
	blob := new(blob.Blob)
	if err := blob.UnmarshalJSON([]byte("{}")); err == nil {
		t.Fatal("expected a non-nil error")
	}
}
