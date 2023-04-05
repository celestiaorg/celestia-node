package blob

import (
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

// Commitment is a Merkle Root of the subtree built from shares of the Blob.
// It is computed by splitting the blob into shares and building the Merkle subtree to be included after Submit.
type Commitment []byte

func (com Commitment) String() string {
	return string(com)
}

// Proof is a collection of nmt.Proofs that verifies the inclusion of the data.
type Proof []*nmt.Proof

// Blob represents any application-specific binary data that anyone can submit to Celestia.
type Blob struct {
	types.Blob

	commitment Commitment
}

// NewBlob constructs a new blob from the provided namespace.ID and data.
func NewBlob(_ uint8, nID namespace.ID, data []byte) (*Blob, error) {
	blob, err := types.NewBlob(nID, data)
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
func (b *Blob) NamespaceID() []byte {
	return b.Blob.NamespaceId
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
