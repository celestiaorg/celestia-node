package header

import (
	"encoding"
	"time"
)

// Header abstracts all methods required to perform header sync.
type Header interface {
	// New creates new instance of a header.
	New() Header
	// IsZero reports whether Header is a zero value of it's concrete type.
	IsZero() bool
	// ChainID returns identifier of the chain (ChainID).
	ChainID() string
	// Hash returns hash of a header.
	Hash() Hash
	// Height returns the height of a header.
	Height() int64
	// LastHeader returns the hash of last header before this header (aka. previous header hash).
	LastHeader() Hash
	// Time returns time when header was created.
	Time() time.Time
	// VerifyAdjacent validates adjacent untrusted header against trusted header.
	VerifyAdjacent(Header) error
	// VerifyNonAdjacent validates non-adjacent untrusted header against trusted header.
	VerifyNonAdjacent(Header) error
	// Validate performs basic validation to check for missed/incorrect fields.
	Validate() error

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}
