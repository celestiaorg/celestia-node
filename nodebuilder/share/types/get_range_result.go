package types

import (
	"bytes"
	"errors"

	"github.com/cometbft/cometbft/crypto/merkle"
	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/types"

	"github.com/celestiaorg/celestia-app/v4/pkg/da"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// GetRangeResult wraps the return value of the GetRange endpoint
// because Json-RPC doesn't support more than two return values.
type GetRangeResult struct {
	Shares []libshare.Share
	Proof  *types.ShareProof
}

func NewGetRangeResult(
	startIndex, endIndex int,
	rngdata *shwap.RangeNamespaceData,
	dah *da.DataAvailabilityHeader,
) (*GetRangeResult, error) {
	ns, err := shwap.ParseNamespace(rngdata.Shares, startIndex, endIndex)
	if err != nil {
		return nil, err
	}

	odsSize := len(dah.RowRoots) / 2
	startRow := startIndex / odsSize
	endRow := (endIndex - 1) / odsSize

	nmtProofs := make([]*nmt.Proof, len(rngdata.Shares))
	nmtProofs[0] = rngdata.FirstIncompleteRowProof
	if startRow != endRow {
		nmtProofs[len(rngdata.Shares)-1] = rngdata.LastIncompleteRowProof
	}
	for i, nmtProof := range nmtProofs {
		if nmtProof != nil {
			continue
		}
		extendedShares, err := share.ExtendShares(rngdata.Shares[i])
		if err != nil {
			return nil, err
		}
		proof, err := shwap.GenerateSharesProofs(
			startRow+i,
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
	rowProofs := make([]*merkle.Proof, endRow-startRow+1)
	rowRoots := make([]tmbytes.HexBytes, endRow-startRow+1)

	for i := startRow; i <= endRow; i++ {
		rowProofs[i-startRow] = &merkle.Proof{
			Total:    allProofs[i].Total,
			Index:    allProofs[i].Index,
			LeafHash: allProofs[i].LeafHash,
			Aunts:    allProofs[i].Aunts,
		}
		rowRoots[i-startRow] = dah.RowRoots[i]
	}

	sharesProof := &types.ShareProof{
		Data:             libshare.ToBytes(rngdata.Flatten()),
		ShareProofs:      coreProofs,
		NamespaceID:      ns.ID(),
		NamespaceVersion: uint32(ns.Version()),
		RowProof: types.RowProof{
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

func toCoreNMTProof(proofs []*nmt.Proof) []*tmproto.NMTProof {
	coreProofs := make([]*tmproto.NMTProof, len(proofs))
	for i, proof := range proofs {
		coreProofs[i] = &tmproto.NMTProof{
			Start:    int32(proof.Start()),
			End:      int32(proof.End()),
			Nodes:    proof.Nodes(),
			LeafHash: proof.LeafHash(),
		}
	}
	return coreProofs
}
