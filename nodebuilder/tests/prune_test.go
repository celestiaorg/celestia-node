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
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

// TestArchivalBlobSync tests whether a LN is able to sync historical blobs from
// an archival node in a network dominated by pruned nodes.
//
// 1 BN w/ pruning, 3 FN w/ pruning, 1 FN archival

// turn on archival BN
// archival FN syncs against BN
// turn off archival BN
// turn on pruning BN
// spin up 3 pruning FNs, connect
// spin up 1 LN that syncs historic blobs
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

	archivalBN := sw.NewBridgeNode()
	sw.SetBootstrapper(t, archivalBN)

	err := archivalBN.Start(ctx)
	require.NoError(t, err)

	archivalFN := sw.NewFullNode()
	err = archivalFN.Start(ctx)
	require.NoError(t, err)

	require.NoError(t, <-fillDn)

	pruningCfg := nodebuilder.DefaultConfig(node.Bridge)
	pruningCfg.Pruner.EnableService = true

	testAvailWindow := pruner.AvailabilityWindow(time.Millisecond)
	prunerOpts := fx.Options(
		fx.Replace(testAvailWindow),
		fxutil.ReplaceAs(func(
			edsClient *shrexeds.Client,
			ndClient *shrexnd.Client,
			managers map[string]*peers.Manager,
		) *getters.ShrexGetter {
			return getters.NewShrexGetter(
				edsClient,
				ndClient,
				managers["full"],
				managers["archival"],
				testAvailWindow,
			)
		}, new(getters.ShrexGetter)),
	)

	// stop the archival BN to force LN to have to discover
	// the archival FN later
	err = archivalBN.Stop(ctx)
	require.NoError(t, err)

	pruningBN := sw.NewNodeWithConfig(node.Bridge, pruningCfg, prunerOpts)
	sw.SetBootstrapper(t, pruningBN)
	err = pruningBN.Start(ctx)
	require.NoError(t, err)

	err = archivalFN.Host.Connect(ctx, *host.InfoFromHost(pruningBN.Host))
	require.NoError(t, err)

	pruningCfg.DASer = das.DefaultConfig(node.Full)
	pruningCfg.Pruner.EnableService = true
	pruningFulls := make([]*nodebuilder.Node, 0, 3)
	for i := 0; i < 3; i++ {
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

	archivalBlobs := make([]*archivalBlob, 0)
	i := 1
	for {
		eh, err := archivalFN.HeaderServ.GetByHeight(ctx, uint64(i))
		require.NoError(t, err)

		if bytes.Equal(eh.DataHash, share.EmptyRoot().Hash()) {
			i++
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

		if len(archivalBlobs) > 10 {
			break
		}
		i++
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
