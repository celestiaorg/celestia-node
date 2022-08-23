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
func Register(proofType ProofType, topic string, unmarshaler ProofUnmarshaler) error {
	proofsLk.Lock()
	if _, ok := supportedProofTypes[proofType]; ok {
		proofsLk.Unlock()
		return errors.New("fraud: proofType is registered")
	}
	supportedProofTypes[proofType] = topic
	proofsLk.Unlock()

	unmarshalersLk.Lock()
	defaultUnmarshalers[proofType] = unmarshaler
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
