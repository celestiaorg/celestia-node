package fraud

var DefaultUnmarshalers = map[ProofType]ProofUnmarshaler{}

func init() {
	Register(BadEncoding, UnmarshalBEFP)
}

func Register(proofType ProofType, unmarshaler ProofUnmarshaler) {
	DefaultUnmarshalers[proofType] = unmarshaler
}
