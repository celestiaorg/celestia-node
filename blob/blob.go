package blob

import (
	"bytes"
	"fmt"

	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

// versionIndex defines the position of the namespace version in the namespace
const versionIndex = 0

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
func NewBlob(shareVersion uint8, namespace namespace.ID, data []byte) (*Blob, error) {
	if len(namespace) != appns.NamespaceSize {
		return nil, fmt.Errorf("invalid size of the namespace id. got:%d, want:%d", len(namespace), appns.NamespaceSize)
	}

	ns, err := appns.New(namespace[versionIndex], namespace[versionIndex+1:])
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

// Namespace returns blob's namespace.
func (b *Blob) Namespace() namespace.ID {
	return append([]byte{uint8(b.NamespaceVersion)}, b.NamespaceId...)
}

// Commitment returns the commitment of current blob.
func (b *Blob) Commitment() Commitment {
	return b.commitment
}
