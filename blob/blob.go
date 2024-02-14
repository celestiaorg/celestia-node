package blob

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
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
// Proof proves the WHOLE namespaced data for the particular row.
// TODO (@vgonkivs): rework `Proof` in order to prove a particular blob.
// https://github.com/celestiaorg/celestia-node/issues/2303
type Proof []*nmt.Proof

func (p Proof) Len() int { return len(p) }

func (p Proof) MarshalJSON() ([]byte, error) {
	proofs := make([]string, 0, len(p))
	for _, proof := range p {
		proofBytes, err := proof.MarshalJSON()
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, string(proofBytes))
	}
	return json.Marshal(proofs)
}

func (p *Proof) UnmarshalJSON(b []byte) error {
	var proofs []string
	if err := json.Unmarshal(b, &proofs); err != nil {
		return err
	}
	for _, proof := range proofs {
		var nmtProof nmt.Proof
		if err := nmtProof.UnmarshalJSON([]byte(proof)); err != nil {
			return err
		}
		*p = append(*p, &nmtProof)
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
	types.Blob `json:"blob"`

	Commitment Commitment `json:"commitment"`

	// the celestia-node's namespace type
	// this is to avoid converting to and from app's type
	namespace share.Namespace

	// index represents index of the first share in the eds.
	// before data is being published, the index set by default to -1.
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

// Index returns the index  of the first share in the eds.
func (b *Blob) Index() int {
	return b.index
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

// index represents index of the blob's first share in the eds.
// -1 by default and only retrieved onchain blobs will have it set.
type transformer struct {
	index  int
	length int
	shares []shares.Share
}

func newTransformer(index, length int) *transformer {
	return &transformer{
		index:  index,
		length: length,
	}
}

// setShares sets shares until the blob is completed and returns extra shares back.
func (b *transformer) setShares(shares []shares.Share) (shrs []shares.Share, isComplete bool) {
	index := -1
	for i, sh := range shares {
		b.shares = append(b.shares, sh)
		if len(b.shares) == b.length {
			index = i
			isComplete = true
			break
		}
	}

	if index == -1 {
		return
	}

	if index+1 >= len(shares) {
		return shrs, true
	}
	return shares[index+1:], true
}

func (b *transformer) transform() (*Blob, error) {
	if b.length != len(b.shares) {
		return nil, errors.New("blob: incomplete blob")
	}

	sequence, err := shares.ParseShares(b.shares, true)
	if err != nil {
		return nil, err
	}

	if len(sequence) != 1 {
		return nil, ErrBlobNotFound
	}

	data, err := sequence[0].RawData()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrBlobNotFound
	}

	shareVersion, err := sequence[0].Shares[0].Version()
	if err != nil {
		return nil, err
	}

	blob, err := NewBlob(shareVersion, sequence[0].Namespace.Bytes(), data)
	if err != nil {
		return nil, err
	}
	blob.index = b.index
	return blob, nil
}

func (b *transformer) isEmpty() bool {
	return b.index == 0 && b.length == 0 && len(b.shares) == 0
}

func (b *transformer) reset() {
	b.index = 0
	b.length = 0
	b.shares = []shares.Share{}
}
