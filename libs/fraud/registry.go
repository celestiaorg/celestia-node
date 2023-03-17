package fraud

import (
	"fmt"
	"reflect"
	"sync"
)

var (
	unmarshalersLk      = sync.RWMutex{}
	defaultUnmarshalers = map[ProofType]ProofUnmarshaler{}
)

// Register adds a string representation and unmarshaller for the provided ProofType.
func Register(p Proof) {
	unmarshalersLk.Lock()
	defer unmarshalersLk.Unlock()
	if _, ok := defaultUnmarshalers[p.Type()]; ok {
		panic(fmt.Sprintf("fraud: unmarshaler for %s proof is registered", p.Type()))
	}
	defaultUnmarshalers[p.Type()] = func(data []byte) (Proof, error) {
		// the underlying type of `p` is a pointer to a struct and assigning `p` to a new variable is not
		// the case, because it could lead to data races.
		// So, there is no easier way to create a hard copy of Proof other than using a reflection.
		proof := reflect.New(reflect.ValueOf(p).Elem().Type()).Interface().(Proof)
		err := proof.UnmarshalBinary(data)
		return proof, err
	}
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
