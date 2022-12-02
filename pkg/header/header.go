package header

import (
	"encoding"
	"time"
)

// Header abstracts all methods required to perform header sync.
type Header interface {
	// New creates new instance of a header.
	New() Header
	// Hash returns hash of a header.
	Hash() Hash
	// Height returns the height of a header.
	Height() int64
	// LastHeader returns the hash of last header before this header (aka. previous header hash).
	LastHeader() Hash
	// Time returns time when header was created.
	Time() time.Time
	// IsRecent checks if header is recent against the given blockTime.
	IsRecent(duration time.Duration) bool
	// IsExpired checks if header is expired against trusting period.
	IsExpired() bool
	// VerifyAdjacent validates adjacent untrusted header against trusted header.
	VerifyAdjacent(Header) error
	// VerifyNonAdjacent validates non-adjacent untrusted header against trusted header.
	VerifyNonAdjacent(Header) error
	// Verify performs basic verification of untrusted header.
	Verify(Header) error
	// Validate performs basic validation to check for missed/incorrect fields.
	Validate() error

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}
