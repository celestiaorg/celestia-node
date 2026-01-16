package store

import (
	"bytes"
	"errors"

	"github.com/ipfs/go-datastore"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/store/file"
)

// NewNamespaceFilter creates a filter that matches a specific namespace.
// Returns nil if the namespace is empty (no filtering).
func NewNamespaceFilter(ns libshare.Namespace) file.NamespaceFilter {
	if ns.IsEmpty() {
		return nil
	}
	return func(other libshare.Namespace) bool {
		return bytes.Equal(other.Bytes(), ns.Bytes())
	}
}

type Parameters struct {
	// RecentBlocksCacheSize is the size of the cache for recent blocks.
	RecentBlocksCacheSize int

	// ---- Pin Node Options (optional, leave zero values for standard nodes) ----

	// TrackedNamespace is the namespace to track for Pin nodes.
	// If set, enables namespace-specific optimizations.
	TrackedNamespace libshare.Namespace `toml:"-"`

	// NamespaceDatastore is used for persistent storage of namespace data.
	// Required if TrackedNamespace is set.
	NamespaceDatastore datastore.Batching `toml:"-"`
}

// DefaultParameters returns the default configuration values for the EDS store parameters.
func DefaultParameters() *Parameters {
	return &Parameters{
		RecentBlocksCacheSize: 10,
	}
}

func (p *Parameters) Validate() error {
	if p.RecentBlocksCacheSize < 0 {
		return errors.New("recent eds cache size cannot be negative")
	}
	return nil
}

// IsNamespaceTrackingEnabled returns true if namespace tracking is configured.
func (p *Parameters) IsNamespaceTrackingEnabled() bool {
	return !p.TrackedNamespace.IsEmpty() && p.NamespaceDatastore != nil
}
