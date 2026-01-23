package blob

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/v7/pkg/appconsts"
	"github.com/celestiaorg/go-square/merkle"
	"github.com/celestiaorg/go-square/v3/inclusion"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var subtreeRootThreshold = appconsts.SubtreeRootThreshold

// The Proof is a set of nmt proofs that can verify the inclusion of the blob
type Proof []*nmt.Proof

func (p Proof) Len() int { return len(p) }

func (p Proof) verify(blob *Blob, header *header.ExtendedHeader) error {
	shrs, err := BlobsToShares(blob)
	if err != nil {
		return err
	}

	fromCoords, err := shwap.SampleCoordsFrom1DIndex(blob.Index(), len(header.DAH.RowRoots)) // pass eds size
	if err != nil {
		return err
	}
	toCoords, err := shwap.SampleCoordsFrom1DIndex(blob.Index()+len(shrs)-1, len(header.DAH.RowRoots))
	if err != nil {
		return err
	}

	hasher := share.NewSHA256Hasher()
	for from, i := fromCoords.Row, 0; from <= toCoords.Row; from++ {
		hasher.Reset()
		sharesPerRow := p[i].End() - p[i].Start()
		valid := p[i].VerifyInclusion(
			hasher,
			blob.Namespace().Bytes(),
			libshare.ToBytes(shrs[i:sharesPerRow]),
			header.DAH.RowRoots[from],
		)
		if !valid {
			return errors.New("invalid share proof")
		}
		i += sharesPerRow
	}
	return nil
}

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
	if err := namespace.ValidateForBlob(); err != nil {
		return nil, fmt.Errorf("invalid user namespace: %w", err)
	}

	libBlob, err := libshare.NewBlob(namespace, data, shareVersion, signer)
	if err != nil {
		return nil, err
	}

	com, err := inclusion.CreateCommitment(libBlob, merkle.HashFromByteSlices, subtreeRootThreshold)
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
	containsSigner := b.ShareVersion() == libshare.ShareVersionOne
	return libshare.SparseSharesNeeded(s[0].SequenceLen(), containsSigner), nil
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
