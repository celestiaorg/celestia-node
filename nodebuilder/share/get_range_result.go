package share

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/crypto/merkle"
	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	tmjson "github.com/cometbft/cometbft/libs/json"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/types"

	"github.com/celestiaorg/celestia-app/v10/pkg/da"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// GetRangeResult wraps the return value of the GetRange endpoint
// because Json-RPC doesn't support more than two return values.
type GetRangeResult struct {
	// Shares contains the data shares retrieved from the specified range.
	Shares []libshare.Share
	// Proof proves that shares were included in the data root.
	Proof *types.ShareProof
}

// newGetRangeResult creates a new GetRangeResult containing namespace data shares
// and their corresponding proofs for a specified range.
//
// The function processes a range of shares from a namespace, generates the necessary
// Merkle proofs and packages everything into a GetRangeResult structure
// that can be used for share verification.
func newGetRangeResult(
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
	numRows := endRow - startRow + 1

	if len(rngdata.Shares) != numRows {
		return nil, errors.New("range data row count does not match requested range")
	}

	nmtProofs := make([]*nmt.Proof, numRows)
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
			odsSize,
			odsSize,
			extendedShares,
		)
		if err != nil {
			return nil, err
		}
		nmtProofs[i] = proof
	}
	coreProofs := toCoreNMTProof(nmtProofs)

	_, allProofs := merkle.ProofsFromByteSlices(append(dah.RowRoots, dah.ColumnRoots...))
	rowProofs := make([]*merkle.Proof, numRows)
	rowRoots := make([]tmbytes.HexBytes, numRows)

	for i := startRow; i <= endRow; i++ {
		rowProofs[i-startRow] = &merkle.Proof{
			Total:    allProofs[i].Total,
			Index:    allProofs[i].Index,
			LeafHash: allProofs[i].LeafHash,
			Aunts:    allProofs[i].Aunts,
		}
		rowRoots[i-startRow] = dah.RowRoots[i]
	}

	data := rngdata.Flatten()
	sharesProof := &types.ShareProof{
		Data:             libshare.ToBytes(data),
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
		Shares: data,
		Proof:  sharesProof,
	}, nil
}

// Verify verifies inclusion the data in the data root
func (r *GetRangeResult) Verify(dataRoot []byte) error {
	if r.Proof == nil {
		return errors.New("nil proof")
	}
	if len(r.Shares) != len(r.Proof.Data) {
		return fmt.Errorf("got %d shares but the proof covers %d", len(r.Shares), len(r.Proof.Data))
	}
	for i, sh := range r.Shares {
		if !bytes.Equal(sh.ToBytes(), r.Proof.Data[i]) {
			return errors.New("share data mismatch")
		}
		ns := sh.Namespace()
		if uint32(ns.Version()) != r.Proof.NamespaceVersion || !bytes.Equal(ns.ID(), r.Proof.NamespaceID) {
			return fmt.Errorf("share %d namespace does not match the proof namespace", i)
		}
	}
	return r.Proof.Validate(dataRoot)
}

// VerifyRange verifies the data like Verify and checks that it is exactly the
// [start, end) range of the original data square.
func (r *GetRangeResult) VerifyRange(dataRoot []byte, start, end int) error {
	if err := r.Verify(dataRoot); err != nil {
		return err
	}
	// Verify checked that the proofs are non-nil and bound to the square.
	width := int(r.Proof.RowProof.Proofs[0].Total / 4)
	first, last := r.Proof.ShareProofs[0], r.Proof.ShareProofs[len(r.Proof.ShareProofs)-1]
	provenStart := int(r.Proof.RowProof.StartRow)*width + int(first.Start)
	provenEnd := int(r.Proof.RowProof.EndRow)*width + int(last.End)
	if provenStart != start || provenEnd != end {
		return fmt.Errorf("proof covers [%d, %d), want [%d, %d)", provenStart, provenEnd, start, end)
	}
	return nil
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

// MarshalJSON marshals an GetRangeResult to JSON. Uses tendermint encoder for proof for compatibility.
func (r *GetRangeResult) MarshalJSON() ([]byte, error) {
	// alias the type to avoid going into recursion loop
	// because tmjson.Marshal invokes custom json marshaling
	type Alias GetRangeResult
	return tmjson.Marshal((*Alias)(r))
}

// UnmarshalJSON unmarshals an GetRangeResult from JSON. Uses tendermint decoder for proof for compatibility.
func (r *GetRangeResult) UnmarshalJSON(data []byte) error {
	// alias the type to avoid going into recursion loop
	// because tmjson.Unmarshal invokes custom json Unmarshaling
	type Alias GetRangeResult
	return tmjson.Unmarshal(data, (*Alias)(r))
}
