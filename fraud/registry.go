package fraud

import (
	"fmt"
	"sync"
)

var unmarshalersLk = sync.RWMutex{}
var defaultUnmarshalers = map[ProofType]ProofUnmarshaler{}

func init() {
	Register(BadEncoding, UnmarshalBEFP)
}

func Register(proofType ProofType, unmarshaler ProofUnmarshaler) {
	unmarshalersLk.Lock()
	unmarshalersLk.Unlock()
	defaultUnmarshalers[proofType] = unmarshaler
}

func GetUnmarshaler(proofType ProofType) (ProofUnmarshaler, error) {
	unmarshalersLk.RLock()
	defer unmarshalersLk.RUnlock()
	unmarshaler, ok := defaultUnmarshalers[proofType]
	if !ok {
		return nil, fmt.Errorf("fraud: unmarshaler for %s type is not registered", proofType)
	}
	return unmarshaler, nil
}
