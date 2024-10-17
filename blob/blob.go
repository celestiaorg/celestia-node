package blob

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/pkg/consts"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	coretypes "github.com/tendermint/tendermint/types"

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

// Proof constructs the proof of a blob to the data root.
type Proof struct {
	// ShareToRowRootProof the proofs of the shares to the row roots they belong to.
	// If the blob spans across multiple rows, then this will contain multiple proofs.
	ShareToRowRootProof []*tmproto.NMTProof
	// RowToDataRootProof the proofs of the row roots containing the blob shares
	// to the data root.
	RowToDataRootProof coretypes.RowProof
}

// namespaceToRowRootProof a proof of a set of namespace shares to the row
// roots they belong to.
type namespaceToRowRootProof []*nmt.Proof

// Verify takes a blob and a data root and verifies if the
// provided blob was committed to the given data root.
func (p *Proof) Verify(blob *Blob, dataRoot []byte) (bool, error) {
	blobCommitment, err := inclusion.CreateCommitment(
		ToAppBlobs(blob)[0],
		merkle.HashFromByteSlices,
		appconsts.DefaultSubtreeRootThreshold,
	)
	if err != nil {
		return false, err
	}
	if !blob.Commitment.Equal(blobCommitment) {
		return false, fmt.Errorf(
			"%w: generated commitment does not match the provided blob commitment",
			ErrMismatchCommitment,
		)
	}
	rawShares, err := BlobsToShares(blob)
	if err != nil {
		return false, err
	}
	return p.VerifyShares(rawShares, blob.namespace, dataRoot)
}

// VerifyShares takes a set of shares, a namespace and a data root, and verifies if the
// provided shares are committed to by the data root.
func (p *Proof) VerifyShares(rawShares [][]byte, namespace share.Namespace, dataRoot []byte) (bool, error) {
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

	if len(jsonBlob.Namespace) == 0 {
		return errors.New("expected a non-empty namespace")
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

// proveRowRootsToDataRoot creates a set of binary merkle proofs for all the
// roots defined by the range [start, end).
func proveRowRootsToDataRoot(roots [][]byte, start, end int) []*merkle.Proof {
	_, proofs := merkle.ProofsFromByteSlices(roots)
	return proofs[start:end]
}
