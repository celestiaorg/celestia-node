package blob

import (
	"bytes"

	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

// Commitment is a Merkle Root of the subtree built from shares of the Blob.
// It is computed by splitting the blob into shares and building the Merkle subtree to be included
// after Submit.
type Commitment []byte

func (com Commitment) String() string {
	return string(com)
}

// Equal ensures that commitments are the same
func (com Commitment) Equal(c Commitment) bool {
	return bytes.Equal(com, c)
}

// Proof is a collection of nmt.Proofs that verifies the inclusion of the data.
type Proof []*nmt.Proof

func (p Proof) Len() int { return len(p) }

// equal is a temporary method that compares two proofs.
// should be removed in BlobService V1.
func (p Proof) equal(input Proof) error {
	if p.Len() != input.Len() {
		return ErrInvalidProof
	}

	for i, proof := range p {
		pNodes := proof.Nodes()
		inputNodes := input[i].Nodes()
		for i, node := range pNodes {
			if !bytes.Equal(node, inputNodes[i]) {
				return ErrInvalidProof
			}
		}

		if proof.Start() != input[i].Start() || proof.End() != input[i].End() {
			return ErrInvalidProof
		}

		if !bytes.Equal(proof.LeafHash(), input[i].LeafHash()) {
			return ErrInvalidProof
		}

	}
	return nil
}

// Blob represents any application-specific binary data that anyone can submit to Celestia.
type Blob struct {
	types.Blob

	commitment Commitment
}

// NewBlob constructs a new blob from the provided namespace.ID and data.
func NewBlob(shareVersion, nIDVersion uint8, nID namespace.ID, data []byte) (*Blob, error) {
	ns, err := appns.New(nIDVersion, nID)
	if err != nil {
		return nil, err
	}

	blob, err := types.NewBlob(ns, data, shareVersion)
	if err != nil {
		return nil, err
	}

	com, err := types.CreateCommitment(blob)
	if err != nil {
		return nil, err
	}
	return &Blob{Blob: *blob, commitment: com}, nil
}

// NamespaceID returns blobs namespaceId.
func (b *Blob) Namespace() appns.Namespace {
	return appns.Namespace{
		ID:      b.Blob.GetNamespaceId(),
		Version: uint8(b.Blob.GetNamespaceVersion()),
	}
}

// Data returns blobs raw data.
func (b *Blob) Data() []byte {
	return b.Blob.Data
}

// Commitment returns the commitment of current blob.
func (b *Blob) Commitment() Commitment {
	return b.commitment
}

// Version is the format that was used to encode Blob data and namespaceID into shares.
func (b *Blob) Version() uint32 {
	return b.Blob.ShareVersion
}
