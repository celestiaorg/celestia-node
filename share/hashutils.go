package share

import (
	"crypto/sha256"
	"hash"
)

// NewSHA256Hasher returns a new instance of a SHA-256 hasher.
func NewSHA256Hasher() hash.Hash {
	return sha256.New()
}
