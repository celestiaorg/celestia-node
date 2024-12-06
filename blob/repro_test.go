package blob

import (
	"testing"

	"github.com/tendermint/tendermint/crypto/merkle"
	coretypes "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/pb"
)

// Reported at https://github.com/celestiaorg/celestia-node/issues/3731.
func TestProofRowProofVerifyWithEmptyRoot(t *testing.T) {
	cp := &Proof{
		RowToDataRootProof: coretypes.RowProof{
			Proofs: []*merkle.Proof{{}},
		},
	}
	root := []byte{0xd3, 0x4d, 0x34}
	if _, err := cp.Verify(root); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

// Reported at https://github.com/celestiaorg/celestia-node/issues/3730.
func TestProofRowProofVerify(t *testing.T) {
	cp := &Proof{
		RowToDataRootProof: coretypes.RowProof{
			Proofs: []*merkle.Proof{{}},
		},
	}
	if _, err := cp.Verify(nil); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

// Reported at https://github.com/celestiaorg/celestia-node/issues/3729.
func TestCommitmentProofVerifySliceBound(t *testing.T) {
	proof := nmt.ProtoToProof(pb.Proof{End: 1})
	cp := &Proof{
		SubtreeRootProofs: []*nmt.Proof{
			&proof,
		},
	}
	if _, err := cp.Verify(nil); err == nil {
		t.Fatal("expected a non-nil error")
	}
}

// Reported at https://github.com/celestiaorg/celestia-node/issues/3727.
func TestBlobUnmarshalRepro(t *testing.T) {
	blob := new(Blob)
	if err := blob.UnmarshalJSON([]byte("{}")); err == nil {
		t.Fatal("expected a non-nil error")
	}
}
