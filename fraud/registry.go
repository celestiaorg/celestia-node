package fraud

import (
	"fmt"
	"sync"
)

var (
	unmarshalersLk      = sync.RWMutex{}
	defaultUnmarshalers = map[ProofType]ProofUnmarshaler{}
)

// Register adds a string representation and unmarshaller for the provided ProofType.
func Register(p ProofType, unmarshaler ProofUnmarshaler) {
	unmarshalersLk.Lock()
	defer unmarshalersLk.Unlock()
	if _, ok := defaultUnmarshalers[p]; ok {
		panic(fmt.Sprintf("fraud: unmarshaler for %s proof is registered", p))
	}
	defaultUnmarshalers[p] = unmarshaler
}

// Registered reports a set of registered proof types by Register.
func Registered() []ProofType {
	return registeredProofTypes()
}

// registeredProofTypes returns all available proofTypes.
func registeredProofTypes() []ProofType {
	unmarshalersLk.Lock()
	defer unmarshalersLk.Unlock()
	proofs := make([]ProofType, 0, len(defaultUnmarshalers))
	for proof := range defaultUnmarshalers {
		proofs = append(proofs, proof)
	}
	return proofs
}
