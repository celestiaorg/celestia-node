package store

import (
	"errors"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/store/file"
)

type Parameters struct {
	// RecentBlocksCacheSize is the size of the cache for recent blocks.
	RecentBlocksCacheSize int
	// NamespaceFilter is a filter for shares to be stored in the EDS store.
	NamespaceFilter file.NamespaceFilter `toml:"-"`
	// NamespaceID is the namespace that the node is tracking.
	NamespaceID libshare.Namespace `toml:"-"`
	// Datastore is used for persistent storage of namespace data.
	Datastore datastore.Batching `toml:"-"`
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
