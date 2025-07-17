//go:build pruning || integration

package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share"
	full_avail "github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
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
	t.Parallel()

	const (
		blocks = 10
		btime  = time.Millisecond * 300
		bsize  = 16
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	heightsCh, fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)

	archivalBN := sw.NewBridgeNode()
	sw.SetBootstrapper(t, archivalBN)

	err := archivalBN.Start(ctx)
	require.NoError(t, err)

	archivalFN := sw.NewFullNode()
	err = archivalFN.Start(ctx)
	require.NoError(t, err)
	sw.SetBootstrapper(t, archivalFN)

	require.NoError(t, <-fillDn)

	heights := make([]uint64, 0, blocks)
	// drain channel to get all heights
	for height := range heightsCh {
		heights = append(heights, height)
	}

	pruningCfg := sw.DefaultTestConfig(node.Bridge)
	pruningCfg.Pruner.EnableService = true

	testAvailWindow := time.Millisecond
	prunerOpts := fx.Options(
		fx.Replace(testAvailWindow),
		fx.Decorate(func(
			client *shrex.Client,
			managers map[string]*peers.Manager,
		) *shrex_getter.Getter {
			return shrex_getter.NewGetter(
				client,
				managers["full"],
				managers["archival"],
				testAvailWindow,
			)
		}),
	)

	// wait until bn syncs to the latest submitted height
	_, err = archivalFN.HeaderServ.WaitForHeight(ctx, heights[len(heights)-1])
	require.NoError(t, err)
	err = archivalBN.Stop(ctx)
	require.NoError(t, err)

	pruningBN := sw.NewNodeWithConfig(node.Bridge, pruningCfg, prunerOpts)
	err = pruningBN.Start(ctx)
	require.NoError(t, err)

	pruningCfg.DASer = das.DefaultConfig(node.Full)
	pruningCfg.Pruner.EnableService = true
	pruningFulls := make([]*nodebuilder.Node, 0, 3)
	for range 3 {
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

	archivalBlobs := make([]*archivalBlob, 0, blocks)
	for _, height := range heights {
		eh, err := archivalFN.HeaderServ.GetByHeight(ctx, uint64(height))
		require.NoError(t, err)
		var ns libshare.Namespace
		for _, roots := range eh.DAH.RowRoots {
			namespace := roots[:libshare.NamespaceSize]
			ns, err = libshare.NewNamespaceFromBytes(namespace)
			require.NoError(t, err)
			// Ideally we should check for `ValidateForBlob` here,
			// but it throws an error every time.
			if ns.IsUsableNamespace() {
				break
			}
		}

		if ns.ID() == nil {
			t.Fatal("usable namespace was not found")
		}

		blobs, err := archivalFN.BlobServ.GetAll(ctx, eh.Height(), []libshare.Namespace{ns})
		require.NoError(t, err)

		archivalBlobs = append(archivalBlobs, &archivalBlob{
			blob:   blobs[0],
			height: eh.Height(),
			root:   eh.DAH.Hash(),
		})
	}

	// ensure pruned FNs don't have the blocks associated
	// with the historical blobs
	for _, pruned := range pruningFulls {
		for _, b := range archivalBlobs {
			has, err := pruned.EDSStore.HasByHeight(ctx, b.height)
			require.NoError(t, err)
			assert.False(t, has)
		}
	}

	ln := sw.NewLightNode(prunerOpts)
	err = ln.Start(ctx)
	require.NoError(t, err)

	// TODO(@Wondertan): A hack that just makes test works
	//  With following pruning intergration PR it just works
	//  and I don't have anymore time to figure this one out
	//  Its something with subscriptions and headers are not
	//  delivered by LN
	go func() {
		for {
			ln.HeaderServ.NetworkHead(ctx)
			time.Sleep(time.Second)
		}
	}()

	// ensure LN can retrieve all archival blobs from the
	// archival FN
	for _, b := range archivalBlobs {
		_, err = ln.HeaderServ.WaitForHeight(ctx, b.height)
		require.NoError(t, err)
		got, err := ln.BlobServ.Get(ctx, b.height, b.blob.Namespace(), b.blob.Commitment)
		require.NoError(t, err)
		assert.Equal(t, b.blob.Commitment, got.Commitment)
		assert.Equal(t, b.blob.Data(), got.Data())
	}
}

func TestDisallowConvertFromPrunedToArchival(t *testing.T) {
	t.Parallel()
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	bootstrapper := sw.NewBridgeNode()
	err := bootstrapper.Start(ctx)
	require.NoError(t, err)
	sw.SetBootstrapper(t, bootstrapper)

	// Light nodes have pruning enabled by default
	for _, nt := range []node.Type{node.Bridge, node.Full} {
		pruningCfg := sw.DefaultTestConfig(nt)
		pruningCfg.Pruner.EnableService = true
		store := nodebuilder.MockStore(t, pruningCfg)
		pruningNode := sw.MustNewNodeWithStore(nt, store)
		err := pruningNode.Start(ctx)
		require.NoError(t, err)
		err = pruningNode.Stop(ctx)
		require.NoError(t, err)

		archivalCfg := sw.DefaultTestConfig(nt)
		err = store.PutConfig(archivalCfg)
		require.NoError(t, err)
		_, err = sw.NewNodeWithStore(nt, store)
		assert.Error(t, err)
		assert.ErrorIs(t, full_avail.ErrDisallowRevertToArchival, err)
	}
}

func TestDisallowConvertToArchivalViaLastPrunedCheck(t *testing.T) {
	t.Parallel()
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	var cp struct {
		LastPrunedHeight uint64              `json:"last_pruned_height"`
		FailedHeaders    map[uint64]struct{} `json:"failed"`
	}

	for _, nt := range []node.Type{node.Bridge, node.Full} {
		archivalCfg := sw.DefaultTestConfig(nt)

		store := nodebuilder.MockStore(t, archivalCfg)
		ds, err := store.Datastore()
		require.NoError(t, err)

		cp.LastPrunedHeight = 500
		cp.FailedHeaders = make(map[uint64]struct{})
		bin, err := json.Marshal(cp)
		require.NoError(t, err)

		prunerStore := namespace.Wrap(ds, datastore.NewKey("pruner"))
		err = prunerStore.Put(ctx, datastore.NewKey("checkpoint"), bin)
		require.NoError(t, err)

		_, err = sw.NewNodeWithStore(nt, store)
		require.Error(t, err)
		assert.ErrorIs(t, full_avail.ErrDisallowRevertToArchival, err)
	}
}

func TestConvertFromArchivalToPruned(t *testing.T) {
	t.Parallel()
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	var cp struct {
		LastPrunedHeight uint64              `json:"last_pruned_height"`
		FailedHeaders    map[uint64]struct{} `json:"failed"`
	}

	bootstrapper := sw.NewBridgeNode()
	err := bootstrapper.Start(ctx)
	require.NoError(t, err)
	sw.SetBootstrapper(t, bootstrapper)

	for _, nt := range []node.Type{node.Bridge, node.Full} {
		archivalCfg := sw.DefaultTestConfig(nt)

		store := nodebuilder.MockStore(t, archivalCfg)
		ds, err := store.Datastore()
		require.NoError(t, err)

		// the archival node has trimmed up to height 500
		fullAvailStore := namespace.Wrap(ds, datastore.NewKey("full_avail"))
		err = fullAvailStore.Put(ctx, datastore.NewKey("previous_mode"), []byte("archival"))
		require.NoError(t, err)

		cp.LastPrunedHeight = 500
		cp.FailedHeaders = make(map[uint64]struct{})
		bin, err := json.Marshal(cp)
		require.NoError(t, err)

		prunerStore := namespace.Wrap(ds, datastore.NewKey("pruner"))
		err = prunerStore.Put(ctx, datastore.NewKey("checkpoint"), bin)
		require.NoError(t, err)

		archivalNode := sw.MustNewNodeWithStore(nt, store)
		err = archivalNode.Start(ctx)
		require.NoError(t, err)
		err = archivalNode.Stop(ctx)
		require.NoError(t, err)

		// convert to pruned node
		pruningCfg := sw.DefaultTestConfig(nt)
		pruningCfg.Pruner.EnableService = true
		err = store.PutConfig(pruningCfg)
		require.NoError(t, err)
		_, err = sw.NewNodeWithStore(nt, store)
		assert.NoError(t, err)

		// expect that the checkpoint has been overridden
		bin, err = prunerStore.Get(ctx, datastore.NewKey("checkpoint"))
		require.NoError(t, err)
		err = json.Unmarshal(bin, &cp)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), cp.LastPrunedHeight)
	}
}
