package prover

import (
	"bytes"
	"errors"
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmcrypto "github.com/tendermint/tendermint/proto/tendermint/crypto"
)

// TODO @renaynay @evan-forbes: this file is CPed from tendermint, it only
//  modifies which decoders get registered, and then changes the api to accept
//  [][]byte instead of a string so we don't have to do any processing on the
//  keypath.

//----------------------------------------
// ProofOp gets converted to an instance of ProofOperator:

// ProofOperator is a layer for calculating intermediate Merkle roots
// when a series of Merkle trees are chained together.
// Run() takes leaf values from a tree and returns the Merkle
// root for the corresponding tree. It takes and returns a list of bytes
// to allow multiple leaves to be part of a single proof, for instance in a range proof.
// ProofOp() encodes the ProofOperator in a generic way so it can later be
// decoded with OpDecoder.
type ProofOperator interface {
	Run([][]byte) ([][]byte, error)
	GetKey() []byte
	ProofOp() tmcrypto.ProofOp
}

//----------------------------------------
// Operations on a list of ProofOperators

// ProofOperators is a slice of ProofOperator(s).
// Each operator will be applied to the input value sequentially
// and the last Merkle root will be verified with already known data
type ProofOperators []ProofOperator

func (poz ProofOperators) VerifyValue(root []byte, keys [][]byte, value []byte) (err error) {
	return poz.Verify(root, keys, [][]byte{value})
}

func (poz ProofOperators) Verify(root []byte, keys [][]byte, args [][]byte) (err error) {
	for i, op := range poz {
		key := op.GetKey()
		if len(key) != 0 {
			if len(keys) == 0 {
				return fmt.Errorf("key path has insufficient # of parts: expected no more keys but got %+v", string(key))
			}
			lastKey := keys[len(keys)-1]
			if !bytes.Equal(lastKey, key) {
				return fmt.Errorf("key mismatch on operation #%d: expected %+v but got %+v", i, string(lastKey), string(key))
			}
			keys = keys[:len(keys)-1]
		}
		args, err = op.Run(args)
		if err != nil {
			return
		}
	}
	if !bytes.Equal(root, args[0]) {
		return fmt.Errorf("calculated root hash is invalid: expected %X but got %X", root, args[0])
	}
	if len(keys) != 0 {
		return errors.New("keypath not consumed all")
	}
	return nil
}

//----------------------------------------
// ProofRuntime - main entrypoint

type ProofRuntime struct {
	decoders map[string]merkle.OpDecoder
}

func NewProofRuntime() *ProofRuntime {
	return &ProofRuntime{
		decoders: make(map[string]merkle.OpDecoder),
	}
}

func (prt *ProofRuntime) RegisterOpDecoder(typ string, dec merkle.OpDecoder) {
	_, ok := prt.decoders[typ]
	if ok {
		panic("already registered for type " + typ)
	}
	prt.decoders[typ] = dec
}

func (prt *ProofRuntime) Decode(pop tmcrypto.ProofOp) (ProofOperator, error) {
	decoder := prt.decoders[pop.Type]
	if decoder == nil {
		return nil, fmt.Errorf("unrecognized proof type %v", pop.Type)
	}
	return decoder(pop)
}

func (prt *ProofRuntime) DecodeProof(proof *tmcrypto.ProofOps) (ProofOperators, error) {
	poz := make(ProofOperators, 0, len(proof.Ops))
	for _, pop := range proof.Ops {
		operator, err := prt.Decode(pop)
		if err != nil {
			return nil, fmt.Errorf("decoding a proof operator: %w", err)
		}
		poz = append(poz, operator)
	}
	return poz, nil
}

func (prt *ProofRuntime) VerifyValue(proof *tmcrypto.ProofOps, root []byte, keys [][]byte, value []byte) (err error) {
	return prt.Verify(proof, root, keys, [][]byte{value})
}

// TODO In the long run we'll need a method of classifcation of ops,
// whether existence or absence or perhaps a third?
func (prt *ProofRuntime) VerifyAbsence(proof *tmcrypto.ProofOps, root []byte, keys [][]byte) (err error) {
	return prt.Verify(proof, root, keys, nil)
}

func (prt *ProofRuntime) Verify(proof *tmcrypto.ProofOps, root []byte, keys [][]byte, args [][]byte) (err error) {
	poz, err := prt.DecodeProof(proof)
	if err != nil {
		return fmt.Errorf("decoding proof: %w", err)
	}
	return poz.Verify(root, keys, args)
}

// DefaultProofRuntime only knows about value proofs.
// To use e.g. IAVL proofs, register op-decoders as
// defined in the IAVL package.
func DefaultProofRuntime() (prt *ProofRuntime) {
	prt = NewProofRuntime()
	prt.RegisterOpDecoder(merkle.ProofOpValue, merkle.ValueOpDecoder)
	prt.RegisterOpDecoder(storetypes.ProofOpIAVLCommitment, storetypes.CommitmentOpDecoder)
	prt.RegisterOpDecoder(storetypes.ProofOpSimpleMerkleCommitment, storetypes.CommitmentOpDecoder)
	return
}
