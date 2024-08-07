package blob

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/v3/pkg/appconsts"
	"github.com/celestiaorg/go-square/merkle"
	"github.com/celestiaorg/go-square/v2/inclusion"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	"github.com/tendermint/tendermint/pkg/consts"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	coretypes "github.com/tendermint/tendermint/types"
)

//nolint:unused
var errEmptyShares = errors.New("empty shares")

// Proof constructs the proof of a blob to the data root.
type Proof struct {
	// ShareToRowRootProof the proofs of the shares to the row roots they belong to.
	// If the blob spans across multiple rows, then this will contain multiple proofs.
	ShareToRowRootProof []*tmproto.NMTProof
	// RowToDataRootProof the proofs of the row roots containing the blob shares
	// to the data root.
	RowToDataRootProof coretypes.RowProof
}

// Verify takes a blob and a data root and verifies if the
// provided blob was committed to the given data root.
func (p *Proof) Verify(blob *Blob, dataRoot []byte) (bool, error) {
	blobCommitment, err := inclusion.CreateCommitment(blob.Blob, merkle.HashFromByteSlices, appconsts.DefaultSubtreeRootThreshold)
	if err != nil {
		return false, err
	}
	if !blob.Commitment.Equal(blobCommitment) {
		return false, fmt.Errorf(
			"%w: generated commitment does not match the provided blob commitment",
			ErrMismatchCommitment,
		)
	}
	shares, err := BlobsToShares(blob)
	if err != nil {
		return false, err
	}
	return p.VerifyShares(libshare.ToBytes(shares), blob.Namespace(), dataRoot)
}

// VerifyShares takes a set of shares, a namespace and a data root, and verifies if the
// provided shares are committed to by the data root.
func (p *Proof) VerifyShares(rawShares [][]byte, namespace libshare.Namespace, dataRoot []byte) (bool, error) {
	// verify the row proof
	if err := p.RowToDataRootProof.Validate(dataRoot); err != nil {
		return false, fmt.Errorf("%w: invalid row root to data root proof", err)
	}

	// verify the share proof
	ns := append([]byte{namespace.Version()}, namespace.ID()...)
	cursor := int32(0)
	for i, proof := range p.ShareToRowRootProof {
		sharesUsed := proof.End - proof.Start
		if len(rawShares) < int(sharesUsed+cursor) {
			return false, fmt.Errorf("%w: invalid number of shares", ErrInvalidProof)
		}
		nmtProof := nmt.NewInclusionProof(
			int(proof.Start),
			int(proof.End),
			proof.Nodes,
			true,
		)
		valid := nmtProof.VerifyInclusion(
			consts.NewBaseHashFunc(),
			ns,
			rawShares[cursor:sharesUsed+cursor],
			p.RowToDataRootProof.RowRoots[i],
		)
		if !valid {
			return false, ErrInvalidProof
		}
		cursor += sharesUsed
	}
	return true, nil
}

// Blob represents any application-specific binary data that anyone can submit to Celestia.
type Blob struct {
	*libshare.Blob `json:"blob"`

	Commitment Commitment `json:"commitment"`

	// index represents the index of the blob's first share in the EDS.
	// Only retrieved, on-chain blobs will have the index set. Default is -1.
	index int
}

// NewBlobV0 constructs a new blob from the provided Namespace and data.
// The blob will be formatted as v0 shares.
func NewBlobV0(namespace libshare.Namespace, data []byte) (*Blob, error) {
	return NewBlob(libshare.ShareVersionZero, namespace, data, nil)
}

// NewBlobV1 constructs a new blob from the provided Namespace, data, and signer.
// The blob will be formatted as v1 shares.
func NewBlobV1(namespace libshare.Namespace, data, signer []byte) (*Blob, error) {
	return NewBlob(libshare.ShareVersionOne, namespace, data, signer)
}

// NewBlob constructs a new blob from the provided Namespace, data, signer, and share version.
func NewBlob(shareVersion uint8, namespace libshare.Namespace, data, signer []byte) (*Blob, error) {
	if len(data) == 0 || len(data) > appconsts.DefaultMaxBytes {
		return nil, fmt.Errorf("blob data must be > 0 && <= %d, but it was %d bytes", appconsts.DefaultMaxBytes, len(data))
	}

	if err := namespace.ValidateForBlob(); err != nil {
		return nil, fmt.Errorf("invalid user namespace: %w", err)
	}

	libBlob, err := libshare.NewBlob(namespace, data, shareVersion, signer)
	if err != nil {
		return nil, err
	}

	com, err := inclusion.CreateCommitment(libBlob, merkle.HashFromByteSlices, appconsts.DefaultSubtreeRootThreshold)
	if err != nil {
		return nil, err
	}
	return &Blob{Blob: libBlob, Commitment: com, index: -1}, nil
}

// Namespace returns blob's namespace.
func (b *Blob) Namespace() libshare.Namespace {
	return b.Blob.Namespace()
}

// Index returns the blob's first share index in the EDS.
// Only retrieved, on-chain blobs will have the index set. Default is -1.
func (b *Blob) Index() int {
	return b.index
}

// Length returns the number of shares in the blob.
func (b *Blob) Length() (int, error) {
	s, err := BlobsToShares(b)
	if err != nil {
		return 0, err
	}

	if len(s) == 0 {
		return 0, errors.New("blob with zero shares received")
	}
	return libshare.SparseSharesNeeded(s[0].SequenceLen()), nil
}

// Signer returns blob's author.
func (b *Blob) Signer() []byte {
	return b.Blob.Signer()
}

func (b *Blob) compareCommitments(com Commitment) bool {
	return bytes.Equal(b.Commitment, com)
}

type jsonBlob struct {
	Namespace    []byte     `json:"namespace"`
	Data         []byte     `json:"data"`
	ShareVersion uint8      `json:"share_version"`
	Commitment   Commitment `json:"commitment"`
	Signer       []byte     `json:"signer,omitempty"`
	Index        int        `json:"index"`
}

func (b *Blob) MarshalJSON() ([]byte, error) {
	blob := &jsonBlob{
		Namespace:    b.Namespace().Bytes(),
		Data:         b.Data(),
		ShareVersion: b.ShareVersion(),
		Commitment:   b.Commitment,
		Signer:       b.Signer(),
		Index:        b.index,
	}
	return json.Marshal(blob)
}

func (b *Blob) UnmarshalJSON(data []byte) error {
	var jsonBlob jsonBlob
	err := json.Unmarshal(data, &jsonBlob)
	if err != nil {
		return err
	}

	ns, err := libshare.NewNamespaceFromBytes(jsonBlob.Namespace)
	if err != nil {
		return err
	}

	blob, err := NewBlob(jsonBlob.ShareVersion, ns, jsonBlob.Data, jsonBlob.Signer)
	if err != nil {
		return err
	}

	blob.Commitment = jsonBlob.Commitment
	blob.index = jsonBlob.Index
	*b = *blob
	return nil
}

// proveRowRootsToDataRoot creates a set of binary merkle proofs for all the
// roots defined by the range [start, end).
func proveRowRootsToDataRoot(roots [][]byte, start, end int) []*merkle.Proof {
	_, proofs := merkle.ProofsFromByteSlices(roots)
	return proofs[start:end]
}
