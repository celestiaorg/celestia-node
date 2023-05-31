package blob

import (
	"bytes"
	"encoding/json"
	"fmt"

	tmjson "github.com/tendermint/tendermint/libs/json"

	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/share/ipld"
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

type jsonProof struct {
	Start int      `json:"start"`
	End   int      `json:"end"`
	Nodes [][]byte `json:"nodes"`
}

func (p *Proof) MarshalJSON() ([]byte, error) {
	proofs := make([]jsonProof, 0, p.Len())
	for _, pp := range *p {
		proofs = append(proofs, jsonProof{
			Start: pp.Start(),
			End:   pp.End(),
			Nodes: pp.Nodes(),
		})
	}

	return json.Marshal(proofs)
}

func (p *Proof) UnmarshalJSON(data []byte) error {
	var proofs []jsonProof
	err := json.Unmarshal(data, &proofs)
	if err != nil {
		return err
	}

	nmtProofs := make([]*nmt.Proof, len(proofs))
	for i, jProof := range proofs {
		nmtProof := nmt.NewInclusionProof(jProof.Start, jProof.End, jProof.Nodes, ipld.NMTIgnoreMaxNamespace)
		nmtProofs[i] = &nmtProof
	}

	*p = nmtProofs
	return nil
}

// Blob represents any application-specific binary data that anyone can submit to Celestia.
type Blob struct {
	types.Blob `json:"blob"`

	Commitment Commitment `json:"commitment"`
}

// NewBlob constructs a new blob from the provided namespace.ID and data.
func NewBlob(shareVersion uint8, namespace namespace.ID, data []byte) (*Blob, error) {
	if len(namespace) != appns.NamespaceSize {
		return nil, fmt.Errorf("invalid size of the namespace id. got:%d, want:%d", len(namespace), appns.NamespaceSize)
	}

	ns, err := appns.New(namespace[appns.NamespaceVersionSize-1], namespace[appns.NamespaceVersionSize:])
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
	return &Blob{Blob: *blob, Commitment: com}, nil
}

// Namespace returns blob's namespace.
func (b *Blob) Namespace() namespace.ID {
	return append([]byte{uint8(b.NamespaceVersion)}, b.NamespaceId...)
}

func (b *Blob) MarshalJSON() ([]byte, error) {
	appBlob := &types.Blob{
		NamespaceId:      b.Namespace(),
		Data:             b.Data,
		ShareVersion:     b.ShareVersion,
		NamespaceVersion: b.NamespaceVersion,
	}
	data, err := tmjson.Marshal(appBlob)
	if err != nil {
		return nil, err
	}

	return json.Marshal(&struct {
		Blob       json.RawMessage `json:"blob"`
		Commitment []byte          `json:"commitment"`
	}{
		Blob:       data,
		Commitment: b.Commitment,
	},
	)
}

func (b *Blob) UnmarshalJSON(data []byte) error {
	aux := &struct {
		Blob       json.RawMessage `json:"blob"`
		Commitment []byte          `json:"commitment"`
	}{}
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return nil
	}

	appBlob := new(types.Blob)
	err = tmjson.Unmarshal(aux.Blob, appBlob)
	if err != nil {
		return err
	}

	appBlob.NamespaceId = appBlob.NamespaceId[1:]
	b.Blob = *appBlob
	b.Commitment = aux.Commitment
	return nil
}
