package blob

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/tendermint/tendermint/pkg/consts"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	coretypes "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
)

var errEmptyShares = errors.New("empty shares")

// Proof constructs the proof of a blob to the data root.
type Proof struct {
	// ShareToRowRootProof the proofs of the shares to the row roots they belong to.
	// If the blob spans across multiple rows, then this will contain multiple proofs.
	ShareToRowRootProof []*tmproto.NMTProof
	// RowProof the proofs of the row roots containing the blob shares
	// to the data root.
	RowProof coretypes.RowProof
}

// equal is a temporary method that compares two proofs.
// should be removed in BlobService V1.
func (p *Proof) equal(input *Proof) error {
	// compare the NMT proofs
	for i, proof := range p.ShareToRowRootProof {
		if proof.Start != input.ShareToRowRootProof[i].Start {
			// TODO(@rach-id): should we define specific errors for each case? that's better
			// to know which part is not equal.
			// Also, this error should be ErrProofNotEqual or similar, and not ErrInvalidProof
			return ErrInvalidProof
		}
		if proof.End != input.ShareToRowRootProof[i].End {
			// TODO(@rach-id): should we define specific errors for each case? that's better
			// to know which part is not equal.
			// Also, this error should be ErrProofNotEqual or similar, and not ErrInvalidProof
			return ErrInvalidProof
		}
		for j, node := range proof.Nodes {
			if !bytes.Equal(node, input.ShareToRowRootProof[i].Nodes[j]) {
				return ErrInvalidProof
			}
		}
	}

	// compare the row proof
	if p.RowProof.StartRow != input.RowProof.StartRow {
		return ErrInvalidProof
	}
	if p.RowProof.EndRow != input.RowProof.EndRow {
		return ErrInvalidProof
	}
	for index, rowRoot := range p.RowProof.RowRoots {
		if !bytes.Equal(rowRoot, input.RowProof.RowRoots[index]) {
			return ErrInvalidProof
		}
	}
	for i, proof := range p.RowProof.Proofs {
		if proof.Index != input.RowProof.Proofs[i].Index {
			return ErrInvalidProof
		}
		if proof.Total != input.RowProof.Proofs[i].Total {
			return ErrInvalidProof
		}
		for j, aunt := range proof.Aunts {
			if !bytes.Equal(aunt, input.RowProof.Proofs[i].Aunts[j]) {
				return ErrInvalidProof
			}
		}
	}
	return nil
}

// TODO(@rach-id): rename to Verify
func (p *Proof) VerifyProof(rawShares [][]byte, namespace share.Namespace, dataRoot []byte) (bool, error) {
	// verify the row proof
	if !p.RowProof.VerifyProof(dataRoot) {
		return false, errors.New("invalid row root to data root proof")
	}

	// verify the share proof
	cursor := int32(0)
	for i, proof := range p.ShareToRowRootProof {
		nmtProof := nmt.NewInclusionProof(
			int(proof.Start),
			int(proof.End),
			proof.Nodes,
			true,
		)
		sharesUsed := proof.End - proof.Start
		if namespace.Version() > math.MaxUint8 {
			return false, errors.New("invalid namespace version")
		}
		ns := append([]byte{namespace.Version()}, namespace.ID()...)
		if len(rawShares) < int(sharesUsed+cursor) {
			return false, errors.New("invalid number of shares")
		}
		valid := nmtProof.VerifyInclusion(
			consts.NewBaseHashFunc(),
			ns,
			rawShares[cursor:sharesUsed+cursor],
			p.RowProof.RowRoots[i],
		)
		if !valid {
			return false, nil
		}
		cursor += sharesUsed
	}
	return true, nil
}

// Blob represents any application-specific binary data that anyone can submit to Celestia.
type Blob struct {
	types.Blob `json:"blob"`

	Commitment Commitment `json:"commitment"`

	// the celestia-node's namespace type
	// this is to avoid converting to and from app's type
	namespace share.Namespace

	// index represents the index of the blob's first share in the EDS.
	// Only retrieved, on-chain blobs will have the index set. Default is -1.
	index int
}

// NewBlobV0 constructs a new blob from the provided Namespace and data.
// The blob will be formatted as v0 shares.
func NewBlobV0(namespace share.Namespace, data []byte) (*Blob, error) {
	return NewBlob(appconsts.ShareVersionZero, namespace, data)
}

// NewBlob constructs a new blob from the provided Namespace, data and share version.
func NewBlob(shareVersion uint8, namespace share.Namespace, data []byte) (*Blob, error) {
	if len(data) == 0 || len(data) > appconsts.DefaultMaxBytes {
		return nil, fmt.Errorf("blob data must be > 0 && <= %d, but it was %d bytes", appconsts.DefaultMaxBytes, len(data))
	}
	if err := namespace.ValidateForBlob(); err != nil {
		return nil, err
	}

	blob := tmproto.Blob{
		NamespaceId:      namespace.ID(),
		Data:             data,
		ShareVersion:     uint32(shareVersion),
		NamespaceVersion: uint32(namespace.Version()),
	}

	com, err := types.CreateCommitment(&blob)
	if err != nil {
		return nil, err
	}
	return &Blob{Blob: blob, Commitment: com, namespace: namespace, index: -1}, nil
}

// Namespace returns blob's namespace.
func (b *Blob) Namespace() share.Namespace {
	return b.namespace
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

	appShare, err := shares.NewShare(s[0])
	if err != nil {
		return 0, err
	}

	seqLength, err := appShare.SequenceLen()
	if err != nil {
		return 0, err
	}

	return shares.SparseSharesNeeded(seqLength), nil
}

func (b *Blob) compareCommitments(com Commitment) bool {
	return bytes.Equal(b.Commitment, com)
}

type jsonBlob struct {
	Namespace    share.Namespace `json:"namespace"`
	Data         []byte          `json:"data"`
	ShareVersion uint32          `json:"share_version"`
	Commitment   Commitment      `json:"commitment"`
	Index        int             `json:"index"`
}

func (b *Blob) MarshalJSON() ([]byte, error) {
	blob := &jsonBlob{
		Namespace:    b.Namespace(),
		Data:         b.Data,
		ShareVersion: b.ShareVersion,
		Commitment:   b.Commitment,
		Index:        b.index,
	}
	return json.Marshal(blob)
}

func (b *Blob) UnmarshalJSON(data []byte) error {
	var blob jsonBlob
	err := json.Unmarshal(data, &blob)
	if err != nil {
		return err
	}

	b.Blob.NamespaceVersion = uint32(blob.Namespace.Version())
	b.Blob.NamespaceId = blob.Namespace.ID()
	b.Blob.Data = blob.Data
	b.Blob.ShareVersion = blob.ShareVersion
	b.Commitment = blob.Commitment
	b.namespace = blob.Namespace
	b.index = blob.Index
	return nil
}
