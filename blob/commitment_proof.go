package blob

import (
	"bytes"
	"fmt"

	coretypes "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/share"
)

// Commitment is a Merkle Root of the subtree built from shares of the Blob.
// It is computed by splitting the blob into shares and building the Merkle subtree to be included
// after Submit.
type Commitment []byte

// CommitmentProof is an inclusion proof of a commitment to the data root.
type CommitmentProof struct {
	// SubtreeRoots are the subtree roots of the blob's data that are
	// used to create the commitment.
	SubtreeRoots [][]byte `json:"subtree_roots"`
	// SubtreeRootProofs are the NMT proofs for the subtree roots
	// to the row roots.
	SubtreeRootProofs []*nmt.Proof `json:"subtree_root_proofs"`
	// NamespaceID is the namespace id of the commitment being proven. This
	// namespace id is used when verifying the proof. If the namespace id doesn't
	// match the namespace of the shares, the proof will fail verification.
	NamespaceID namespace.ID `json:"namespace_id"`
	// RowProof is the proof of the rows containing the blob's data to the
	// data root.
	RowProof         coretypes.RowProof `json:"row_proof"`
	NamespaceVersion uint8              `json:"namespace_version"`
}

func (com Commitment) String() string {
	return string(com)
}

// Equal ensures that commitments are the same
func (com Commitment) Equal(c Commitment) bool {
	return bytes.Equal(com, c)
}

// Validate performs basic validation to the commitment proof.
// Note: it doesn't verify if the proof is valid or not.
// Check Verify() for that.
func (commitmentProof *CommitmentProof) Validate() error {
	if len(commitmentProof.SubtreeRoots) < len(commitmentProof.SubtreeRootProofs) {
		return fmt.Errorf(
			"the number of subtree roots %d should be bigger than the number of subtree root proofs %d",
			len(commitmentProof.SubtreeRoots),
			len(commitmentProof.SubtreeRootProofs),
		)
	}
	if len(commitmentProof.SubtreeRootProofs) != len(commitmentProof.RowProof.Proofs) {
		return fmt.Errorf(
			"the number of subtree root proofs %d should be equal to the number of row root proofs %d",
			len(commitmentProof.SubtreeRootProofs),
			len(commitmentProof.RowProof.Proofs),
		)
	}
	if int(commitmentProof.RowProof.EndRow-commitmentProof.RowProof.StartRow+1) != len(commitmentProof.RowProof.RowRoots) {
		return fmt.Errorf(
			"the number of rows %d must equal the number of row roots %d",
			int(commitmentProof.RowProof.EndRow-commitmentProof.RowProof.StartRow+1),
			len(commitmentProof.RowProof.RowRoots),
		)
	}
	if len(commitmentProof.RowProof.Proofs) != len(commitmentProof.RowProof.RowRoots) {
		return fmt.Errorf(
			"the number of proofs %d must equal the number of row roots %d",
			len(commitmentProof.RowProof.Proofs),
			len(commitmentProof.RowProof.RowRoots),
		)
	}
	return nil
}

// Verify verifies that a commitment proof is valid, i.e., the subtree roots commit
// to some data that was posted to a square.
// Expects the commitment proof to be properly formulated and validated
// using the Validate() function.
func (commitmentProof *CommitmentProof) Verify(root []byte, subtreeRootThreshold int) (bool, error) {
	nmtHasher := nmt.NewNmtHasher(appconsts.NewBaseHashFunc(), share.NamespaceSize, true)

	// computes the total number of shares proven.
	numberOfShares := 0
	for _, proof := range commitmentProof.SubtreeRootProofs {
		numberOfShares += proof.End() - proof.Start()
	}

	// use the computed total number of shares to calculate the subtree roots
	// width.
	// the subtree roots width is defined in ADR-013:
	//
	//https://github.com/celestiaorg/celestia-app/blob/main/docs/architecture/adr-013-non-interactive-default-rules-for-zero-padding.md
	subtreeRootsWidth := shares.SubTreeWidth(numberOfShares, subtreeRootThreshold)

	// verify the proof of the subtree roots
	subtreeRootsCursor := 0
	for i, subtreeRootProof := range commitmentProof.SubtreeRootProofs {
		// calculate the share range that each subtree root commits to.
		ranges, err := nmt.ToLeafRanges(subtreeRootProof.Start(), subtreeRootProof.End(), subtreeRootsWidth)
		if err != nil {
			return false, err
		}
		valid, err := subtreeRootProof.VerifySubtreeRootInclusion(
			nmtHasher,
			commitmentProof.SubtreeRoots[subtreeRootsCursor:subtreeRootsCursor+len(ranges)],
			subtreeRootsWidth,
			commitmentProof.RowProof.RowRoots[i],
		)
		if err != nil {
			return false, err
		}
		if !valid {
			return false,
				fmt.Errorf(
					"subtree root proof for range [%d, %d) is invalid",
					subtreeRootProof.Start(),
					subtreeRootProof.End(),
				)
		}
		subtreeRootsCursor += len(ranges)
	}

	// verify row roots to data root proof
	return commitmentProof.RowProof.VerifyProof(root), nil
}
