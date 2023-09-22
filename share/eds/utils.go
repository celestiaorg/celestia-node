package eds

import (
	"fmt"
	"io"

	"github.com/filecoin-project/dagstore"

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

// blockstoreCloser constructs new BlockstoreCloser from cache.Accessor
func blockstoreCloser(ac cache.Accessor) (*BlockstoreCloser, error) {
	bs, err := ac.Blockstore()
	if err != nil {
		return nil, fmt.Errorf("eds/store: failed to get blockstore: %w", err)
	}
	return &BlockstoreCloser{
		ReadBlockstore: bs,
		Closer:         ac,
	}, nil
}

func closeAndLog(name string, closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Warnw("closing "+name, "err", err)
	}
}
