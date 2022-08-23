package fraud

import (
	"errors"
	"fmt"
	"sync"
)

var (
	unmarshalersLk      = sync.RWMutex{}
	defaultUnmarshalers = map[ProofType]ProofUnmarshaler{}

	proofsLk            = sync.RWMutex{}
	supportedProofTypes = map[ProofType]string{}
)

// Register sets supported proofs with it string representation and unmarshalers in maps by provided ProofType.
func Register(p Proof) error {
	proofsLk.Lock()
	if _, ok := supportedProofTypes[p.Type()]; ok {
		proofsLk.Unlock()
		return errors.New("fraud: proofType is registered")
	}
	supportedProofTypes[p.Type()] = p.Name()
	proofsLk.Unlock()

	unmarshalersLk.Lock()
	defaultUnmarshalers[p.Type()] = func(data []byte) (Proof, error) {
		err := p.UnmarshalBinary(data)
		return p, err
	}
	unmarshalersLk.Unlock()
	return nil
}

// GetTopic returns string representation of topic by provided ProofType.
func GetTopic(proofType ProofType) (string, error) {
	proofsLk.RLock()
	topic, ok := supportedProofTypes[proofType]
	proofsLk.RUnlock()
	if !ok {
		return "", fmt.Errorf("fraud: proof %d is not supported", proofType)
	}
	return topic, nil
}
