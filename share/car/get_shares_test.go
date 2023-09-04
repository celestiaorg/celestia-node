package car

import (
	"bytes"
	"context"
	"crypto/sha256"
	"sort"
	"testing"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

type testNamespacedShares struct {
	ns     share.Namespace
	shares [][]byte
}

func TestReadSharesByNamespace(t *testing.T) {
	// context with timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// initialize eds.Store with random EDS data
	store, err := newStore(t)
	require.NoError(t, err)

	// create a random root
	namespaces := []share.Namespace{
		sharetest.RandV0Namespace(),
		sharetest.RandV0Namespace(),
	}

	nsShares, eds, dah := randEDSWithNamespaces(t, 4, namespaces...)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	// write EDS to store
	err = store.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	// read shares by namespace
	for _, ns := range namespaces {
		shares, err := ReadSharesByNamespace(ctx, store, &dah, ns)
		require.NoError(t, err)

		assert.Equal(t, 2, len(shares))

		nsshares := nsShares[string(ns)].shares
		assert.Equal(t, len(nsshares), len(shares.Flatten()))

		require.NoError(t, shares.Verify(&dah, ns))

		for i, shr := range shares.Flatten() {
			assert.Equal(t, share.GetNamespace(nsshares[i]), share.GetNamespace(shr))
			assert.Equal(t, nsshares[i], shr)
		}

		rowRoots, err := eds.RowRoots()
		require.NoError(t, err)

		roots := make([][]byte, 0)
		for _, root := range rowRoots {
			if ns.IsOutsideRange(root, root) {
				continue
			}
			roots = append(roots, root)
		}

		rows := []share.NamespacedRow(shares)
		for i, row := range rows {
			proof := row.Proof.VerifyInclusion(
				sha256.New(),
				ns.ToNMT(),
				row.Shares,
				roots[i],
			)
			assert.True(t, proof)

			leaves := make([][]byte, len(row.Shares))
			for i := range leaves {
				shr := row.Shares[i]
				leaves[i] = append(share.GetNamespace(shr), shr...)
			}

			proof = row.Proof.VerifyNamespace(
				sha256.New(),
				ns.ToNMT(),
				leaves,
				roots[i],
			)
			assert.True(t, proof)
		}
	}

	err = store.Stop(ctx)
	require.NoError(t, err)
}

// TODO: we need to generate a random EDS with multiple namespaces
func randEDSWithNamespaces(
	t require.TestingT,
	size int,
	namespaces ...share.Namespace,
) (namespacedShares map[string]testNamespacedShares, eds *rsmt2d.ExtendedDataSquare, dah da.DataAvailabilityHeader) {
	shares := make([][]byte, 0)
	sharesSize := (size * size) / len(namespaces)
	namespacedShares = map[string]testNamespacedShares{}
	for _, ns := range namespaces {
		nsShares := sharetest.RandSharesWithNamespace(t, ns, sharesSize)
		namespacedShares[string(ns)] = testNamespacedShares{ns, nsShares}
		shares = append(shares, nsShares...)
	}
	sort.Slice(shares, func(i, j int) bool { return bytes.Compare(shares[i], shares[j]) < 0 })
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(size)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")
	dah, err = da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)
	return
}

func newStore(t *testing.T) (*eds.Store, error) {
	t.Helper()

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	return eds.NewStore(tmpDir, ds)
}
