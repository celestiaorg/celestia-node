package fraud

import (
	"sync"
)

var (
	unmarshalersLk      = sync.RWMutex{}
	defaultUnmarshalers = map[ProofType]ProofUnmarshaler{}
)

// Register sets unmarshaler in map by provided ProofType.
func Register(proofType ProofType, unmarshaler ProofUnmarshaler) {
	unmarshalersLk.Lock()
	defer unmarshalersLk.Unlock()
	defaultUnmarshalers[proofType] = unmarshaler
}
