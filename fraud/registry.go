package fraud

import (
	"fmt"
	"reflect"
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
		// the underlying type of `p` is a pointer to a struct and assigning `p` to a new variable is not the
		// case, because it could lead to data races.
		// So, there is no easier way to create a hard copy of Proof other than using a reflection.
		proof := reflect.New(reflect.ValueOf(p).Elem().Type()).Interface().(Proof)
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
