//go:build pruning || integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share"
	full_avail "github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexeds"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd"
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
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)

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

	testAvailWindow := time.Millisecond
	prunerOpts := fx.Options(
		fx.Replace(testAvailWindow),
		fxutil.ReplaceAs(func(
			edsClient *shrexeds.Client,
			ndClient *shrexnd.Client,
			managers map[string]*peers.Manager,
		) *shrex_getter.Getter {
			return shrex_getter.NewGetter(
				edsClient,
				ndClient,
				managers["full"],
				managers["archival"],
				testAvailWindow,
			)
		}, new(shrex_getter.Getter)),
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

		if bytes.Equal(eh.DataHash, share.EmptyEDSRoots().Hash()) {
			i++
			continue
		}

		shr, err := archivalFN.ShareServ.GetShare(ctx, eh.Height(), 2, 2)
		require.NoError(t, err)

		blobs, err := archivalFN.BlobServ.GetAll(ctx, eh.Height(), []libshare.Namespace{shr.Namespace()})
		require.NoError(t, err)

		archivalBlobs = append(archivalBlobs, &archivalBlob{
			blob:   blobs[0],
			height: eh.Height(),
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
			has, err := pruned.EDSStore.HasByHeight(ctx, b.height)
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
		assert.Equal(t, b.blob.Data(), got.Data())
	}
}

func TestDisallowConvertFromPrunedToArchival(t *testing.T) {
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	// Light nodes have pruning enabled by default
	for _, nt := range []node.Type{node.Bridge, node.Full} {
		pruningCfg := nodebuilder.DefaultConfig(nt)
		pruningCfg.Pruner.EnableService = true
		var err error
		pruningCfg.Core.IP, pruningCfg.Core.Port, err = net.SplitHostPort(sw.ClientContext.GRPCClient.Target())
		require.NoError(t, err)
		store := nodebuilder.MockStore(t, pruningCfg)
		pruningNode := sw.MustNewNodeWithStore(nt, store)
		err = pruningNode.Start(ctx)
		require.NoError(t, err)
		err = pruningNode.Stop(ctx)
		require.NoError(t, err)

		archivalCfg := nodebuilder.DefaultConfig(nt)
		err = store.PutConfig(archivalCfg)
		require.NoError(t, err)
		_, err = sw.NewNodeWithStore(nt, store)
		assert.Error(t, err)
		assert.ErrorIs(t, full_avail.ErrDisallowRevertToArchival, err)
	}
}

func TestDisallowConvertToArchivalViaLastPrunedCheck(t *testing.T) {
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	var cp struct {
		LastPrunedHeight uint64              `json:"last_pruned_height"`
		FailedHeaders    map[uint64]struct{} `json:"failed"`
	}

	for _, nt := range []node.Type{node.Bridge, node.Full} {
		archivalCfg := nodebuilder.DefaultConfig(nt)

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
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	var cp struct {
		LastPrunedHeight uint64              `json:"last_pruned_height"`
		FailedHeaders    map[uint64]struct{} `json:"failed"`
	}

	host, port, err := net.SplitHostPort(sw.ClientContext.GRPCClient.Target())
	require.NoError(t, err)

	for _, nt := range []node.Type{node.Bridge, node.Full} {
		archivalCfg := nodebuilder.DefaultConfig(nt)
		archivalCfg.Core.IP = host
		archivalCfg.Core.Port = port

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
		pruningCfg := nodebuilder.DefaultConfig(nt)
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
