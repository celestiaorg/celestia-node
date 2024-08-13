package blob

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/merkle"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	v2 "github.com/celestiaorg/celestia-app/v2/pkg/appconsts/v2"
	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/inclusion"
	"github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
)

// appVersion is the current application version of celestia-app.
const appVersion = v2.Version

var errEmptyShares = errors.New("empty shares")

// The Proof is a set of nmt proofs that can be verified only through
// the included method (due to limitation of the nmt https://github.com/celestiaorg/nmt/issues/218).
// Proof proves the WHOLE namespaced data to the row roots.
// TODO (@vgonkivs): rework `Proof` in order to prove a particular blob.
// https://github.com/celestiaorg/celestia-node/issues/2303
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
	*blob.Blob `json:"blob"`

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

	blob := blob.Blob{
		NamespaceId:      namespace.ID(),
		Data:             data,
		ShareVersion:     uint32(shareVersion),
		NamespaceVersion: uint32(namespace.Version()),
	}

	com, err := inclusion.CreateCommitment(&blob, merkle.HashFromByteSlices, appconsts.SubtreeRootThreshold(appVersion))
	if err != nil {
		return nil, err
	}
	return &Blob{Blob: &blob, Commitment: com, namespace: namespace, index: -1}, nil
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
	var jsonBlob jsonBlob
	err := json.Unmarshal(data, &jsonBlob)
	if err != nil {
		return err
	}

	b.Blob = &blob.Blob{}
	b.Blob.NamespaceVersion = uint32(jsonBlob.Namespace.Version())
	b.Blob.NamespaceId = jsonBlob.Namespace.ID()
	b.Blob.Data = jsonBlob.Data
	b.Blob.ShareVersion = jsonBlob.ShareVersion
	b.Commitment = jsonBlob.Commitment
	b.namespace = jsonBlob.Namespace
	b.index = jsonBlob.Index
	return nil
}
