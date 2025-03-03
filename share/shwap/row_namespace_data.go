package shwap

import (
	"errors"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-app/v3/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v3/pkg/wrapper"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// RowNamespaceDataName is the name identifier for the row namespace data container.
const RowNamespaceDataName = "rnd_v0"

// ErrNamespaceOutsideRange is returned by RowNamespaceDataFromShares when the target namespace is
// outside of the namespace range for the given row. In this case, the implementation cannot return
// the non-inclusion proof and will return ErrNamespaceOutsideRange.
var ErrNamespaceOutsideRange = errors.New("target namespace is outside of namespace range for the given root")

// RowNamespaceData holds shares and their corresponding proof for a single row within a namespace.
type RowNamespaceData struct {
	Shares []libshare.Share `json:"shares"` // Shares within the namespace.
	Proof  *Proof           `json:"proof"`  // Proof of the shares' inclusion in the namespace.
}

// RowNamespaceDataFromShares extracts and constructs a RowNamespaceData from shares within the
// specified namespace.
func RowNamespaceDataFromShares(
	shares []libshare.Share,
	namespace libshare.Namespace,
	rowIndex int,
	roots *share.AxisRoots,
) (RowNamespaceData, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares)/2), uint(rowIndex))
	nmtTree := nmt.New(
		appconsts.NewBaseHashFunc(),
		nmt.NamespaceIDSize(libshare.NamespaceSize),
		nmt.IgnoreMaxNamespace(true),
	)
	tree.SetTree(nmtTree)

	for _, shr := range shares {
		if err := tree.Push(shr.ToBytes()); err != nil {
			return RowNamespaceData{}, fmt.Errorf("failed to build tree for row %d: %w", rowIndex, err)
		}
	}

	root, err := tree.Root()
	if err != nil {
		return RowNamespaceData{}, fmt.Errorf("failed to get root for row %d: %w", rowIndex, err)
	}
	outside, err := share.IsOutsideRange(namespace, root, root)
	if err != nil {
		return RowNamespaceData{}, err
	}
	if outside {
		return RowNamespaceData{}, ErrNamespaceOutsideRange
	}

	var from, count int
	for i := range len(shares) / 2 {
		if namespace.Equals(shares[i].Namespace()) {
			if count == 0 {
				from = i
			}
			count++
			continue
		}
		if count > 0 {
			break
		}
	}

	var (
		sharesProof nmt.Proof
		rowShares   []libshare.Share
	)

	// if count is 0, then the namespace is not present in the shares. Return non-inclusion proof.
	if count == 0 {
		sharesProof, err = nmtTree.ProveNamespace(namespace.Bytes())
		if err != nil {
			return RowNamespaceData{}, fmt.Errorf("failed to generate non-inclusion proof for row %d: %w", rowIndex, err)
		}
	} else {
		sharesProof, err = tree.ProveRange(from, from+count)
		if err != nil {
			return RowNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", rowIndex, err)
		}
		rowShares = shares[from : from+count]
	}
	prof := RowNamespaceData{
		Shares: rowShares,
		Proof:  NewProof(rowIndex, &sharesProof, roots),
	}

	return prof, prof.Verify(roots.Hash(), namespace, rowIndex)
}

// RowNamespaceDataFromProto constructs RowNamespaceData out of its protobuf representation.
func RowNamespaceDataFromProto(row *pb.RowNamespaceData) (RowNamespaceData, error) {
	shares, err := SharesFromProto(row.GetShares())
	if err != nil {
		return RowNamespaceData{}, err
	}

	proof, err := ProofFromProto(row.Proof)
	if err != nil {
		return RowNamespaceData{}, err
	}
	return RowNamespaceData{
		Shares: shares,
		Proof:  proof,
	}, nil
}

// ToProto converts RowNamespaceData to its protobuf representation for serialization.
func (rnd RowNamespaceData) ToProto() *pb.RowNamespaceData {
	return &pb.RowNamespaceData{
		Shares: SharesToProto(rnd.Shares),
		Proof:  rnd.Proof.ToProto(),
	}
}

// IsEmpty reports whether the RowNamespaceData is empty, i.e. doesn't contain a proof.
func (rnd RowNamespaceData) IsEmpty() bool {
	return rnd.Proof.IsEmptyProof()
}

// Verify checks validity of the RowNamespaceData against the AxisRoots, Namespace and Row index.
func (rnd RowNamespaceData) Verify(dataRoot []byte, namespace libshare.Namespace, rowIdx int) error {
	if rnd.Proof == nil || rnd.Proof.IsEmptyProof() {
		return fmt.Errorf("nil proof")
	}
	if len(rnd.Shares) == 0 && !rnd.Proof.shareProof.IsOfAbsence() {
		return fmt.Errorf("empty shares with non-absence proof for row %d", rowIdx)
	}
	if len(rnd.Shares) > 0 && rnd.Proof.shareProof.IsOfAbsence() {
		return fmt.Errorf("non-empty shares with absence proof for row %d", rowIdx)
	}

	err := rnd.Proof.VerifyNamespace(rnd.Shares, namespace, dataRoot)
	if err != nil {
		return fmt.Errorf("%w for row: %d, %w", ErrFailedVerification, rowIdx, err)
	}
	return nil
}

// ReadFrom reads length-delimited protobuf representation of RowNamespaceData
// implementing io.ReaderFrom.
func (rnd *RowNamespaceData) ReadFrom(reader io.Reader) (int64, error) {
	var pbrnd pb.RowNamespaceData
	n, err := serde.Read(reader, &pbrnd)
	if err != nil {
		return int64(n), fmt.Errorf("reading RowNamespaceData: %w", err)
	}

	*rnd, err = RowNamespaceDataFromProto(&pbrnd)
	return int64(n), err
}

// WriteTo writes length-delimited protobuf representation of RowNamespaceData.
// implementing io.WriterTo.
func (rnd RowNamespaceData) WriteTo(writer io.Writer) (int64, error) {
	pbrnd := rnd.ToProto()
	n, err := serde.Write(writer, pbrnd)
	if err != nil {
		return int64(n), fmt.Errorf("writing RowNamespaceData: %w", err)
	}

	return int64(n), nil
}
