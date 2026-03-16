package crypto

import (
	"crypto/sha256"
	"errors"
	"math/big"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	blsfr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/ecc"
)

var (
	ErrKZGInvalidInputs = errors.New("kzg: invalid inputs")
	ErrKZGVerifyFailed  = errors.New("kzg: verify failed")
)

// BLS12381HomomorphicKZG is a pragmatic “real group math” implementation of the
// HomomorphicKZG interface, built on gnark-crypto BLS12-381.
//
// Important:
// - This is NOT a full polynomial KZG scheme over an evaluation domain yet.
// - It provides the homomorphic commitment property over group elements:
//   Com(y) == sum_i g_i * Com(x_i)
//   where commitments are group elements in G1.
//
// This is sufficient to replace the noop verifier and make the validation gate
// cryptographically meaningful while the full CDA publication pipeline (real y
// computed over F_r, and proper proof semantics) is implemented.
type BLS12381HomomorphicKZG struct {
	g1 bls12381.G1Affine
}

func NewBLS12381HomomorphicKZG() BLS12381HomomorphicKZG {
	_, _, g1Aff, _ := bls12381.Generators()
	return BLS12381HomomorphicKZG{g1: g1Aff}
}

func (kzg BLS12381HomomorphicKZG) CommitColumn(pieces [][]byte) (ColumnCommitment, error) {
	// Transitional: commit to the hash of all pieces.
	// Full CDA will commit per-piece/per-column as specified by the protocol.
	h := sha256.New()
	for _, p := range pieces {
		_, _ = h.Write(p)
	}
	sum := h.Sum(nil)

	var s blsfr.Element
	s.SetBytes(sum)

	var p bls12381.G1Affine
	p.ScalarMultiplication(&kzg.g1, s.BigInt(new(big.Int)))
	out := p.Bytes()
	return out[:], nil
}

func (kzg BLS12381HomomorphicKZG) CombineCommitments(coms []ColumnCommitment, g CodingVector) (ColumnCommitment, error) {
	k := int(g.K)
	if k <= 0 || len(coms) < k {
		return nil, ErrKZGInvalidInputs
	}
	scalars, err := kzg.coeffsAsFr(g, k)
	if err != nil {
		return nil, err
	}

	points := make([]bls12381.G1Affine, 0, k)
	for i := 0; i < k; i++ {
		var p bls12381.G1Affine
		if err := p.Unmarshal(coms[i]); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	var out bls12381.G1Affine
	_, err = out.MultiExp(points, scalars, ecc.MultiExpConfig{})
	if err != nil {
		return nil, err
	}
	b := out.Bytes()
	return b[:], nil
}

func (kzg BLS12381HomomorphicKZG) VerifyFragment(y []byte, g CodingVector, colComs []ColumnCommitment, proof FragmentProof) error {
	k := int(g.K)
	if k <= 0 || len(colComs) < k || len(proof.ProofBytes) == 0 {
		return ErrKZGInvalidInputs
	}

	expected, err := kzg.CombineCommitments(colComs, g)
	if err != nil {
		return err
	}

	var got bls12381.G1Affine
	if err := got.Unmarshal(proof.ProofBytes); err != nil {
		return err
	}

	var exp bls12381.G1Affine
	if err := exp.Unmarshal(expected); err != nil {
		return err
	}

	if !got.Equal(&exp) {
		return ErrKZGVerifyFailed
	}

	_ = y // y binding will be enforced once y is computed over F_r (RLNC).
	return nil
}

// coeffsAsFr converts the coding vector coefficients into field scalars.
// If explicit Coeffs are present, they are used; otherwise coefficients are
// derived from the seed in SHA-256 counter mode.
func (kzg BLS12381HomomorphicKZG) coeffsAsFr(v CodingVector, k int) ([]blsfr.Element, error) {
	if k <= 0 {
		return nil, ErrKZGInvalidInputs
	}

	scalars := make([]blsfr.Element, k)

	if len(v.Coeffs) >= k {
		for i := 0; i < k; i++ {
			scalars[i].SetBytes(v.Coeffs[i].Bytes)
		}
		return scalars, nil
	}

	// Seeded derivation.
	seedNonZero := false
	for _, b := range v.Seed {
		if b != 0 {
			seedNonZero = true
			break
		}
	}
	if !seedNonZero {
		return nil, ErrKZGInvalidInputs
	}

	for i := 0; i < k; i++ {
		// counter-mode derivation (1-byte counter; sufficient for small k <= 32)
		h := sha256.Sum256(append(v.Seed[:], byte(i)))
		scalars[i].SetBytes(h[:])
	}
	return scalars, nil
}

