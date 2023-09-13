package eds

import (
	"io"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"

	"github.com/celestiaorg/celestia-node/share/eds/cache"
)

// readCloser is a helper struct, that combines io.Reader and io.Closer
type readCloser struct {
	io.Reader
	io.Closer
}

// BlockstoreCloser represents a blockstore that can also be closed. It combines the functionality
// of a dagstore.ReadBlockstore with that of an io.Closer.
type BlockstoreCloser struct {
	dagstore.ReadBlockstore
	io.Closer
}

func newReadCloser(ac cache.Accessor) io.ReadCloser {
	return readCloser{
		ac.Reader(),
		ac,
	}
}

func newBlockstore(store *Store, ds datastore.Batching) *blockstore {
	return &blockstore{
		store: store,
		ds:    namespace.Wrap(ds, blockstoreCacheKey),
	}
}

func closeAndLog(name string, closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Warnw("closing "+name, "err", err)
	}
}
