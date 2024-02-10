package eds

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/boxo/blockservice"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/cache"
	"github.com/celestiaorg/celestia-node/share/ipld"
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

// RetrieveNamespaceFromStore gets all EDS shares in the given namespace from
// the EDS store through the corresponding CAR-level blockstore. It is extracted
// from the store getter to make it available for reuse in the shrexnd server.
func RetrieveNamespaceFromStore(
	ctx context.Context,
	store *Store,
	dah *share.Dah,
	namespace share.Namespace,
) (shares share.NamespacedShares, err error) {
	if err = namespace.ValidateForData(); err != nil {
		return nil, err
	}

	bs, err := store.CARBlockstore(ctx, dah.Hash())
	if errors.Is(err, ErrNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve blockstore from eds store: %w", err)
	}
	defer func() {
		if err := bs.Close(); err != nil {
			log.Warnw("closing blockstore", "err", err)
		}
	}()

	// wrap the read-only CAR blockstore in a getter
	blockGetter := NewBlockGetter(bs)
	shares, err = CollectSharesByNamespace(ctx, blockGetter, dah, namespace)
	if errors.Is(err, ipld.ErrNodeNotFound) {
		// IPLD node not found after the index pointed to this shard and the CAR
		// blockstore has been opened successfully is a strong indicator of
		// corruption. We remove the block on bridges and fulls and return
		// share.ErrNotFound to ensure the data is retrieved by the next getter.
		// Note that this recovery is manual and will only be restored by an RPC
		// call to SharesAvailable that fetches the same datahash that was
		// removed.
		err = store.Remove(ctx, dah.Hash())
		if err != nil {
			log.Errorf("failed to remove CAR from store after detected corruption: %w", err)
		}
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve shares by namespace from store: %w", err)
	}

	return shares, nil
}

// CollectSharesByNamespace collects NamespaceShares within the given namespace from share.Root.
func CollectSharesByNamespace(
	ctx context.Context,
	bg blockservice.BlockGetter,
	root *share.Dah,
	namespace share.Namespace,
) (shares share.NamespacedShares, err error) {
	ctx, span := tracer.Start(ctx, "collect-shares-by-namespace", trace.WithAttributes(
		attribute.String("namespace", namespace.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	rootCIDs := ipld.FilterRootByNamespace(root, namespace)
	if len(rootCIDs) == 0 {
		return []share.NamespacedRow{}, nil
	}

	errGroup, ctx := errgroup.WithContext(ctx)
	shares = make([]share.NamespacedRow, len(rootCIDs))
	for i, rootCID := range rootCIDs {
		// shadow loop variables, to ensure correct values are captured
		i, rootCID := i, rootCID
		errGroup.Go(func() error {
			row, proof, err := ipld.GetSharesByNamespace(ctx, bg, rootCID, namespace, len(root.RowRoots))
			shares[i] = share.NamespacedRow{
				Shares: row,
				Proof:  proof,
			}
			if err != nil {
				return fmt.Errorf("retrieving shares by namespace %s for row %x: %w", namespace.String(), rootCID, err)
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, err
	}

	return shares, nil
}
