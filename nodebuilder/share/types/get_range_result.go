package types

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/crypto/merkle"
	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/types"

	"github.com/celestiaorg/celestia-app/v4/pkg/da"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/proof"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// GetRangeResult wraps the return value of the GetRange endpoint
// because Json-RPC doesn't support more than two return values.
type GetRangeResult struct {
	Shares []libshare.Share
	Proof  *types.ShareProof
}

func NewGetRangeResult(
	rngdata *shwap.RangeNamespaceData,
	start, end int,
	dah *da.DataAvailabilityHeader,
) (*GetRangeResult, error) {
	ns, err := parseNamespace(rngdata.Flatten(), start, end)
	if err != nil {
		return nil, err
	}

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
	rowProofs := make([]*merkle.Proof, endRow-startRow+1)
	rowRoots := make([]tmbytes.HexBytes, len(rowProofs))
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
		Data:             rawShares,
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

// parseNamespace validates the share range, checks if it only contains one namespace and returns
// that namespace ID.
// The provided range, defined by startShare and endShare, is end-exclusive.
func parseNamespace(rawShares []libshare.Share, startShare, endShare int) (libshare.Namespace, error) {
	if startShare < 0 {
		return libshare.Namespace{}, fmt.Errorf("start share %d should be positive", startShare)
	}

	if endShare < 0 {
		return libshare.Namespace{}, fmt.Errorf("end share %d should be positive", endShare)
	}

	if endShare <= startShare {
		return libshare.Namespace{}, fmt.Errorf(
			"end share %d cannot be lower or equal to the starting share %d", endShare, startShare,
		)
	}

	if endShare-startShare != len(rawShares) {
		return libshare.Namespace{}, fmt.Errorf(
			"end share %d is higher than block shares %d", endShare, len(rawShares),
		)
	}

	startShareNs := rawShares[startShare].Namespace()
	for i, sh := range rawShares {
		ns := sh.Namespace()
		if !bytes.Equal(startShareNs.Bytes(), ns.Bytes()) {
			return libshare.Namespace{}, fmt.Errorf(
				"shares range contain different namespaces at index %d: %v and %v ", i, startShareNs, ns,
			)
		}
	}
	return startShareNs, nil
}
