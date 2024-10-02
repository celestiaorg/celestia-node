package eds

import (
	"github.com/tendermint/tendermint/crypto/merkle"
	corebytes "github.com/tendermint/tendermint/libs/bytes"
	coretypes "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"

	pkgproof "github.com/celestiaorg/celestia-app/v2/pkg/proof"
	"github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/rsmt2d"
)

// ProveShares generates a share proof for a share range.
// The share range, defined by start and end, is end-exclusive.
func ProveShares(eds *rsmt2d.ExtendedDataSquare, start, end int) (*types.ShareProof, error) {
	log.Debugw("proving share range", "start", start, "end", end)

	odsShares, err := shares.FromBytes(eds.FlattenedODS())
	if err != nil {
		return nil, err
	}
	nID, err := pkgproof.ParseNamespace(odsShares, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("generating the share proof", "start", start, "end", end)
	proof, err := pkgproof.NewShareInclusionProofFromEDS(eds, nID, shares.NewRange(start, end))
	if err != nil {
		return nil, err
	}
	coreProof := toCoreShareProof(proof)
	return &coreProof, nil
}

// toCoreShareProof utility function that converts a share proof defined in app
// to the share proof defined in node.
// This will be removed once we unify both these proofs.
// Reference issue: https://github.com/celestiaorg/celestia-app/issues/3734
func toCoreShareProof(appShareProof pkgproof.ShareProof) types.ShareProof {
	shareProofs := make([]*coretypes.NMTProof, 0)
	for _, proof := range appShareProof.ShareProofs {
		shareProofs = append(shareProofs, &coretypes.NMTProof{
			Start:    proof.Start,
			End:      proof.End,
			Nodes:    proof.Nodes,
			LeafHash: proof.LeafHash,
		})
	}

	rowRoots := make([]corebytes.HexBytes, 0)
	rowProofs := make([]*merkle.Proof, 0)
	for index, proof := range appShareProof.RowProof.Proofs {
		rowRoots = append(rowRoots, appShareProof.RowProof.RowRoots[index])
		rowProofs = append(rowProofs, &merkle.Proof{
			Total:    proof.Total,
			Index:    proof.Index,
			LeafHash: proof.LeafHash,
			Aunts:    proof.Aunts,
		})
	}

	return types.ShareProof{
		Data:        appShareProof.Data,
		ShareProofs: shareProofs,
		NamespaceID: appShareProof.NamespaceId,
		RowProof: types.RowProof{
			RowRoots: rowRoots,
			Proofs:   rowProofs,
			StartRow: appShareProof.RowProof.StartRow,
			EndRow:   appShareProof.RowProof.EndRow,
		},
		NamespaceVersion: appShareProof.NamespaceVersion,
	}
}
