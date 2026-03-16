package crypto

// ColumnCommitment represents a KZG commitment to a column of pieces.
// The concrete format is determined by the underlying KZG library.
type ColumnCommitment []byte

// FieldElement represents an element of the finite field used for RLNC and KZG.
// This is intentionally opaque at this layer; concrete implementations should
// provide constructors and serialization helpers as needed.
type FieldElement struct {
	// Encoded field element bytes (implementation-specific).
	Bytes []byte
}

// CodingVector holds the RLNC coefficients used to produce a coded fragment.
//
// Representation:
// - If Seed is set, the coefficients are derived deterministically from the seed
//   (and any domain separation the caller applies) to reduce wire/storage footprint.
// - Coeffs may be populated directly (explicit form) or used as a cache after derivation.
type CodingVector struct {
	// K is the number of coefficients in the vector.
	K uint16
	// Seed is an optional 32-byte seed for deterministic coefficient derivation.
	// When all bytes are zero, callers may treat the vector as explicit.
	Seed [32]byte
	Coeffs []FieldElement
}

// FragmentProof wraps the proof bytes for a single coded fragment.
type FragmentProof struct {
	ProofBytes []byte
}

// HomomorphicKZG defines the minimal interface required by CDA to:
//   - commit to columns of pieces,
//   - combine commitments homomorphically given a coding vector, and
//   - verify coded fragments against column commitments.
//
// Implementations are expected to be backed by a KZG library such as gnark-crypto.
type HomomorphicKZG interface {
	// CommitColumn computes a KZG commitment to a column of pieces.
	CommitColumn(pieces [][]byte) (ColumnCommitment, error)

	// CombineCommitments returns the commitment to y = sum g_i x_i given
	// column commitments com_i and coding vector g.
	CombineCommitments(coms []ColumnCommitment, g CodingVector) (ColumnCommitment, error)

	// VerifyFragment checks that a coded fragment y with coding vector g
	// is consistent with the provided column commitments, using the supplied proof.
	VerifyFragment(y []byte, g CodingVector, colComs []ColumnCommitment, proof FragmentProof) error
}

