package crypto

import "errors"

// NoopHomomorphicKZG is a development/testing implementation.
//
// It DOES NOT provide real cryptographic security. It exists to unblock
// integration of the CDA network/storage pipeline before wiring a real
// gnark-crypto backed KZG implementation.
type NoopHomomorphicKZG struct{}

func (NoopHomomorphicKZG) CommitColumn(_ [][]byte) (ColumnCommitment, error) {
	return nil, nil
}

func (NoopHomomorphicKZG) CombineCommitments(_ []ColumnCommitment, _ CodingVector) (ColumnCommitment, error) {
	return nil, nil
}

func (NoopHomomorphicKZG) VerifyFragment(_ []byte, g CodingVector, _ []ColumnCommitment, _ FragmentProof) error {
	// Basic shape validation only.
	if g.K == 0 {
		return errors.New("noop kzg: missing coding vector K")
	}
	return nil
}

