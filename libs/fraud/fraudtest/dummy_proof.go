package fraudtest

import (
	"encoding/json"
	"errors"

	"github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/libs/fraud"
)

func init() {
	fraud.Register(&DummyProof{})
}

type DummyProof struct {
	Valid bool
}

func NewValidProof() *DummyProof {
	return &DummyProof{true}
}

func NewInvalidProof() *DummyProof {
	return &DummyProof{false}
}

func (m *DummyProof) Type() fraud.ProofType {
	return "DummyProof"
}

func (m *DummyProof) HeaderHash() []byte {
	return []byte("hash")
}

func (m *DummyProof) Height() uint64 {
	return 1
}

func (m *DummyProof) Validate(header.Header) error {
	if !m.Valid {
		return errors.New("DummyProof: proof is not valid")
	}
	return nil
}

func (m *DummyProof) MarshalBinary() (data []byte, err error) {
	return json.Marshal(m)
}

func (m *DummyProof) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, m)
}
