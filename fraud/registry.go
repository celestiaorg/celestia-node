package fraud

import (
	"fmt"
	"sync"
)

var (
	unmarshalersLk      = sync.RWMutex{}
	defaultUnmarshalers = map[ProofType]ProofUnmarshaler{}

	proofsLk            = sync.RWMutex{}
	supportedProofTypes = map[ProofType]string{}
)

// Register adds a string representation and unmarshaller for the provided ProofType.
func Register(p Proof) {
	proofsLk.Lock()
	if _, ok := supportedProofTypes[p.Type()]; ok {
		proofsLk.Unlock()
		panic("fraud: proofType is registered")
	}
	supportedProofTypes[p.Type()] = p.Name()
	proofsLk.Unlock()

	unmarshalersLk.Lock()
	defaultUnmarshalers[p.Type()] = func(data []byte) (Proof, error) {
		proof := p
		err := proof.UnmarshalBinary(data)
		return proof, err
	}
	unmarshalersLk.Unlock()
}

// getTopic returns string representation of topic by provided ProofType.
func getTopic(proofType ProofType) (string, error) {
	proofsLk.RLock()
	topic, ok := supportedProofTypes[proofType]
	proofsLk.RUnlock()
	if !ok {
		return "", fmt.Errorf("fraud: proof %d is not supported", proofType)
	}
	return topic, nil
}
