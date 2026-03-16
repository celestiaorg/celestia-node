package crypto

import (
	"math/big"
	"testing"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	blsfr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/stretchr/testify/require"
)

func commitScalar(t *testing.T, kzg BLS12381HomomorphicKZG, s blsfr.Element) ColumnCommitment {
	t.Helper()
	var p bls12381.G1Affine
	p.ScalarMultiplication(&kzg.g1, s.BigInt(new(big.Int)))
	b := p.Bytes()
	return b[:]
}

func TestBLS12381HomomorphicKZG_CombineAndVerify(t *testing.T) {
	kzg := NewBLS12381HomomorphicKZG()

	// Two committed values x0, x1.
	var x0, x1 blsfr.Element
	x0.SetUint64(3)
	x1.SetUint64(9)
	com0 := commitScalar(t, kzg, x0)
	com1 := commitScalar(t, kzg, x1)

	// coding vector g = [2, 5]
	var g0, g1 blsfr.Element
	g0.SetUint64(2)
	g1.SetUint64(5)

	cv := CodingVector{
		K: 2,
		Coeffs: []FieldElement{
			func() FieldElement { b := g0.Bytes(); return FieldElement{Bytes: b[:]} }(),
			func() FieldElement { b := g1.Bytes(); return FieldElement{Bytes: b[:]} }(),
		},
	}

	expected, err := kzg.CombineCommitments([]ColumnCommitment{com0, com1}, cv)
	require.NoError(t, err)

	// Proof carries Com(y) for this transitional verifier.
	err = kzg.VerifyFragment([]byte("unused"), cv, []ColumnCommitment{com0, com1}, FragmentProof{ProofBytes: expected})
	require.NoError(t, err)

	// Wrong proof must fail.
	err = kzg.VerifyFragment([]byte("unused"), cv, []ColumnCommitment{com0, com1}, FragmentProof{ProofBytes: com0})
	require.Error(t, err)
}

