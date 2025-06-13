package types

import (
	"bytes"
	"errors"

	pkgProof "github.com/celestiaorg/celestia-app/v4/pkg/proof"
	"github.com/celestiaorg/go-square/merkle"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/proof"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// GetRangeResult wraps the return value of the GetRange endpoint
// because Json-RPC doesn't support more than two return values.
type GetRangeResult struct {
	Shares []libshare.Share
	Proof  *pkgProof.ShareProof
}

func NewGetRangeResult(rngdata *shwap.RangeNamespaceData, header *header.ExtendedHeader) (*GetRangeResult, error) {
	rawShares := make([][]byte, 0, rngdata.EndRow+1)
	for _, shares := range rngdata.Shares {
		rawShares = append(rawShares, libshare.ToBytes(shares)...)
	}

	nmtProofs := make([]*nmt.Proof, rngdata.EndRow+1)
	if rngdata.Proof != nil {
		if rngdata.Proof.FirstIncompleteRowProof != nil {
			nmtProofs[0] = rngdata.Proof.FirstIncompleteRowProof
		}
		if rngdata.Proof.LastIncompleteRowProof != nil {
			nmtProofs[rngdata.EndRow] = rngdata.Proof.LastIncompleteRowProof
		}
	}

	startRow := rngdata.StartRow
	endRow := rngdata.EndRow
	dah := header.DAH
	for i, nmtProof := range nmtProofs {
		if nmtProof != nil {
			continue
		}
		extendedShares, err := share.ExtendShares(rngdata.Shares[i])
		if err != nil {
			return nil, err
		}
		proof, err := proof.GenerateSharesProofs(
			int(startRow)+i,
			0,
			len(dah.RowRoots)/2,
			len(dah.RowRoots)/2,
			extendedShares,
		)
		if err != nil {
			return nil, err
		}
		nmtProofs[i] = proof
	}
	coreProofs := toCoreNMTProof(nmtProofs)

	_, allProofs := merkle.ProofsFromByteSlices(append(dah.RowRoots, dah.ColumnRoots...))
	rowProofs := make([]*pkgProof.Proof, endRow-startRow+1)
	rowRoots := make([][]byte, len(rowProofs))
	for i := startRow; i <= endRow; i++ {
		rowProofs[i-startRow] = &pkgProof.Proof{
			Total:    allProofs[i].Total,
			Index:    allProofs[i].Index,
			LeafHash: allProofs[i].LeafHash,
			Aunts:    allProofs[i].Aunts,
		}
		rowRoots[i-startRow] = dah.RowRoots[i]
	}

	sharesProof := &pkgProof.ShareProof{
		Data:             rawShares,
		ShareProofs:      coreProofs,
		NamespaceId:      rngdata.Shares[0][0].Namespace().ID(),
		NamespaceVersion: uint32(rngdata.Shares[0][0].Namespace().Version()),
		RowProof: &pkgProof.RowProof{
			RowRoots: rowRoots,
			Proofs:   rowProofs,
			StartRow: uint32(startRow),
			EndRow:   uint32(endRow),
		},
	}

	return &GetRangeResult{
		Shares: rngdata.Flatten(),
		Proof:  sharesProof,
	}, nil
}

// Verify verifies inclusion the data in the data root
func (r *GetRangeResult) Verify(dataRoot []byte) error {
	rawShares := libshare.ToBytes(r.Shares)
	for i, shares := range rawShares {
		if !bytes.Equal(shares, r.Proof.Data[i]) {
			return errors.New("share data mismatch")
		}
	}

	if !r.Proof.VerifyProof() {
		return errors.New("proof verification failed")
	}
	return r.Proof.Validate(dataRoot)
}

func toCoreNMTProof(proofs []*nmt.Proof) []*pkgProof.NMTProof {
	coreProofs := make([]*pkgProof.NMTProof, len(proofs))
	for i, proof := range proofs {
		coreProofs[i] = &pkgProof.NMTProof{
			Start:    int32(proof.Start()),
			End:      int32(proof.End()),
			Nodes:    proof.Nodes(),
			LeafHash: proof.LeafHash(),
		}
	}
	return coreProofs
}
