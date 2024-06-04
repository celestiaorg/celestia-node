package tests

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share"
)

// TestArchivalBlobSync tests whether a LN is able to sync historical blobs from
// an archival node in a network dominated by pruned nodes.
//
// 1 BN w/ pruning, 3 FN w/ pruning, 1 FN archival
//
// Steps:
// 1. turn on archival BN
// 2. archival FN syncs against BN
// 3. turn off archival BN
// 4. turn on pruning BN
// 5. spin up 3 pruning FNs, connect
// 6. spin up 1 LN that syncs historic blobs
func TestArchivalBlobSync(t *testing.T) {
	const (
		blocks = 50
		btime  = time.Millisecond * 300
		bsize  = 16
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	// step 1.
	archivalBN := sw.NewBridgeNode()
	sw.SetBootstrapper(t, archivalBN)

	err := archivalBN.Start(ctx)
	require.NoError(t, err)

	// step 2.
	archivalFN := sw.NewFullNode()
	err = archivalFN.Start(ctx)
	require.NoError(t, err)

	require.NoError(t, <-fillDn)

	// step 3.
	// stop the archival BN to force LN to have to discover
	// the archival FN later
	err = archivalBN.Stop(ctx)
	require.NoError(t, err)

	pruningCfg := nodebuilder.DefaultConfig(node.Bridge)
	pruningCfg.Pruner.EnableService = true

	testAvailWindow := pruner.AvailabilityWindow(time.Millisecond)
	prunerOpts := fx.Options(
		fx.Replace(testAvailWindow),
	)

	// step 4.
	pruningBN := sw.NewNodeWithConfig(node.Bridge, pruningCfg, prunerOpts)
	sw.SetBootstrapper(t, pruningBN)
	err = pruningBN.Start(ctx)
	require.NoError(t, err)

	err = archivalFN.Host.Connect(ctx, *host.InfoFromHost(pruningBN.Host))
	require.NoError(t, err)

	// step 5.
	const numFNs = 3
	pruningCfg.DASer = das.DefaultConfig(node.Full)
	pruningCfg.Pruner.EnableService = true
	pruningFulls := make([]*nodebuilder.Node, 0)
	for i := 0; i < numFNs; i++ {
		pruningFN := sw.NewNodeWithConfig(node.Full, pruningCfg, prunerOpts)
		err = pruningFN.Start(ctx)
		require.NoError(t, err)

		pruningFulls = append(pruningFulls, pruningFN)
	}

	type archivalBlob struct {
		blob   *blob.Blob
		height uint64
		root   share.DataHash
	}

	const wantBlobs = 10
	archivalBlobs := make([]*archivalBlob, 0)

	for i := 1; len(archivalBlobs) < wantBlobs; i++ {
		eh, err := archivalFN.HeaderServ.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		if bytes.Equal(eh.DataHash, share.EmptyRoot().Hash()) {
			continue
		}

		shr, err := archivalFN.ShareServ.GetShare(ctx, eh, 2, 2)
		require.NoError(t, err)
		ns, err := share.NamespaceFromBytes(shr[:share.NamespaceSize])
		require.NoError(t, err)

		blobs, err := archivalFN.BlobServ.GetAll(ctx, uint64(i), []share.Namespace{ns})
		require.NoError(t, err)

		archivalBlobs = append(archivalBlobs, &archivalBlob{
			blob:   blobs[0],
			height: uint64(i),
			root:   eh.DAH.Hash(),
		})
	}

	// ensure pruned FNs don't have the blocks associated
	// with the historical blobs
	for _, pruned := range pruningFulls {
		for _, b := range archivalBlobs {
			has, err := pruned.EDSStore.Has(ctx, b.root)
			require.NoError(t, err)
			assert.False(t, has)
		}
	}

	// step 6.
	ln := sw.NewLightNode(prunerOpts)
	err = ln.Start(ctx)
	require.NoError(t, err)

	// ensure LN can retrieve all archival blobs from the
	// archival FN
	for _, b := range archivalBlobs {
		_, err = ln.HeaderServ.WaitForHeight(ctx, b.height)
		require.NoError(t, err)
		got, err := ln.BlobServ.Get(ctx, b.height, b.blob.Namespace(), b.blob.Commitment)
		require.NoError(t, err)
		assert.Equal(t, b.blob.Commitment, got.Commitment)
		assert.Equal(t, b.blob.Data, got.Data)
	}
}

// Pruning_FN from only archival_FN.
//
// Steps:
// 1. turn on archival FN
// 2. pruning FN syncs against archival FN
// 3. spin up 1 LN that syncs historic blobs
func TestPruningFNBlobSync(t *testing.T) {
	const (
		blocks = 50
		btime  = 300 * time.Millisecond
		bsize  = 16
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	// step 1.
	archivalBN := sw.NewBridgeNode()
	sw.SetBootstrapper(t, archivalBN)

	err := archivalBN.Start(ctx)
	require.NoError(t, err)

	// step 2.
	pruningCfg := nodebuilder.DefaultConfig(node.Full)
	pruningCfg.Pruner.EnableService = true

	testAvailWindow := pruner.AvailabilityWindow(time.Millisecond)
	prunerOpts := fx.Options(
		fx.Replace(testAvailWindow),
	)

	pruningFN := sw.NewNodeWithConfig(node.Full, pruningCfg, prunerOpts)
	sw.SetBootstrapper(t, pruningFN)

	err = pruningFN.Start(ctx)
	require.NoError(t, err)

	require.NoError(t, <-fillDn)

	type archivalBlob struct {
		blob   *blob.Blob
		height uint64
		root   share.DataHash
	}

	const wantBlobs = 10
	archivalBlobs := make([]*archivalBlob, 0)

	for i := 1; len(archivalBlobs) < wantBlobs; i++ {
		eh, err := pruningFN.HeaderServ.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		if bytes.Equal(eh.DataHash, share.EmptyRoot().Hash()) {
			continue
		}

		shr, err := pruningFN.ShareServ.GetShare(ctx, eh, 2, 2)
		require.NoError(t, err)
		ns, err := share.NamespaceFromBytes(shr[:share.NamespaceSize])
		require.NoError(t, err)

		blobs, err := pruningFN.BlobServ.GetAll(ctx, uint64(i), []share.Namespace{ns})
		require.NoError(t, err)

		archivalBlobs = append(archivalBlobs, &archivalBlob{
			blob:   blobs[0],
			height: uint64(i),
			root:   eh.DAH.Hash(),
		})
	}

	// step 3.
	ln := sw.NewLightNode(prunerOpts)
	err = ln.Start(ctx)
	require.NoError(t, err)

	// ensure LN can retrieve all blobs from the pruning FN
	for _, b := range archivalBlobs {
		_, err := ln.HeaderServ.WaitForHeight(ctx, b.height)
		require.NoError(t, err)

		got, err := ln.BlobServ.Get(ctx, b.height, b.blob.Namespace(), b.blob.Commitment)
		require.NoError(t, err)
		assert.Equal(t, b.blob.Commitment, got.Commitment)
		assert.Equal(t, b.blob.Data, got.Data)
	}
}
