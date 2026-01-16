package utils

import (
	"encoding/hex"
	"fmt"

	libshare "github.com/celestiaorg/go-square/v3/share"
)

// ParseNamespace parses a namespace from a hex string.
// It supports both:
//   - Full 58-character hex (29 bytes): version byte + 28 byte ID
//   - Short hex (up to 20 characters / 10 bytes): user portion only, left-padded as v0
func ParseNamespace(id string) (libshare.Namespace, error) {
	if id == "" {
		return libshare.Namespace{}, nil
	}

	decoded, err := hex.DecodeString(id)
	if err != nil {
		return libshare.Namespace{}, fmt.Errorf("invalid hex: %w", err)
	}

	// Full namespace (29 bytes = 1 version + 28 ID)
	if len(decoded) == libshare.NamespaceSize {
		return libshare.NewNamespaceFromBytes(decoded)
	}

	// Short format: treat as user portion of a v0 namespace (max 10 bytes)
	if len(decoded) <= 10 {
		return libshare.NewV0Namespace(decoded)
	}

	return libshare.Namespace{}, fmt.Errorf(
		"invalid namespace length: got %d bytes, expected %d (full) or <= 10 (short v0)",
		len(decoded), libshare.NamespaceSize,
	)
}

