package fraud

import (
	"github.com/celestiaorg/go-fraud"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
)

var defaultProofUnmarshaler proofRegistry

type proofRegistry struct{}

func (pr proofRegistry) List() []fraud.ProofType {
	return []fraud.ProofType{
		byzantine.BadEncoding,
	}
}

func (pr proofRegistry) Unmarshal(proofType fraud.ProofType, data []byte) (fraud.Proof[*header.ExtendedHeader], error) {
	switch proofType {
	case byzantine.BadEncoding:
		befp := &byzantine.BadEncodingProof{}
		err := befp.UnmarshalBinary(data)
		if err != nil {
			return nil, err
		}
		return befp, nil
	default:
		return nil, &fraud.ErrNoUnmarshaler{ProofType: proofType}
	}
}
