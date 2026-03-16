package share

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"

	"github.com/celestiaorg/celestia-node/share/crypto"
)

// NewSeededCodingVector creates a seeded coding vector of length k.
// Coefficients can be derived deterministically from the seed when needed.
func NewSeededCodingVector(k uint16) (crypto.CodingVector, error) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return crypto.CodingVector{}, err
	}
	return crypto.CodingVector{
		K:    k,
		Seed: seed,
	}, nil
}

// MaterializeCodingVectorCoeffs deterministically derives coefficients from the
// coding vector seed if Coeffs is empty. If Coeffs is already populated, it is
// returned as-is.
//
// This uses SHA-256 in counter mode to avoid extra dependencies; cryptographic
// strength of the PRG can be revisited when integrating gnark-crypto field ops.
func MaterializeCodingVectorCoeffs(v crypto.CodingVector) crypto.CodingVector {
	if len(v.Coeffs) > 0 {
		return v
	}
	if v.K == 0 {
		return v
	}

	coeffs := make([]crypto.FieldElement, 0, int(v.K))
	var ctr [8]byte
	for i := uint16(0); i < v.K; i++ {
		binary.BigEndian.PutUint64(ctr[:], uint64(i))
		h := sha256.Sum256(append(v.Seed[:], ctr[:]...))
		coeffs = append(coeffs, crypto.FieldElement{Bytes: h[:]})
	}
	v.Coeffs = coeffs
	return v
}

