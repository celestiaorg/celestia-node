package tests

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

func TestBlobModuleGet(t *testing.T) {
	const (
		btime = time.Millisecond * 300
	)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)
	blob := blob.GenerateBlobs(t, []int{16}, false)
	bridge := sw.NewBridgeNode()
	require.NoError(t, bridge.Start(ctx))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	fullCfg := sw.DefaultTestConfig(node.Full)
	fullCfg.Header.TrustedPeers = append(fullCfg.Header.TrustedPeers, addrs[0].String())
	fullNode := sw.NewNodeWithConfig(node.Full, fullCfg)
	require.NoError(t, fullNode.Start(ctx))

	height, err := fullNode.BlobServ.Submit(ctx, blob...)
	require.NoError(t, err)
	_, err = fullNode.HeaderServ.WaitForHeight(ctx, height)
	require.NoError(t, err)

	blob1, err := fullNode.BlobServ.Get(ctx, height, blob[0].NamespaceID(), blob[0].Commitment())
	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(blob[0], blob1))
}

func TestBlobModuleIncluded(t *testing.T) {
	const (
		btime = time.Millisecond * 300
	)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)
	blob := blob.GenerateBlobs(t, []int{16}, false)
	bridge := sw.NewBridgeNode()
	require.NoError(t, bridge.Start(ctx))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	fullCfg := sw.DefaultTestConfig(node.Full)
	fullCfg.Header.TrustedPeers = append(fullCfg.Header.TrustedPeers, addrs[0].String())
	fullNode := sw.NewNodeWithConfig(node.Full, fullCfg)
	require.NoError(t, fullNode.Start(ctx))

	addrsFull, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(fullNode.Host))
	require.NoError(t, err)

	lightCfg := sw.DefaultTestConfig(node.Light)
	lightCfg.Header.TrustedPeers = append(lightCfg.Header.TrustedPeers, addrsFull[0].String())
	lightNode := sw.NewNodeWithConfig(node.Light, lightCfg)
	require.NoError(t, lightNode.Start(ctx))

	height, err := fullNode.BlobServ.Submit(ctx, blob...)
	require.NoError(t, err)
	_, err = fullNode.HeaderServ.WaitForHeight(ctx, height)
	require.NoError(t, err)

	_, err = lightNode.HeaderServ.WaitForHeight(ctx, height)
	require.NoError(t, err)

	proof, err := fullNode.BlobServ.GetProof(ctx, height, blob[0].NamespaceID(), blob[0].Commitment())
	require.NoError(t, err)

	included, err := lightNode.BlobServ.Included(ctx, height, blob[0].NamespaceID(), proof, blob[0].Commitment())
	require.NoError(t, err)
	require.True(t, included)
}

func TestBlobModuleGetAll(t *testing.T) {
	const (
		btime = time.Millisecond * 300
	)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)
	blobs := blob.GenerateBlobs(t, []int{8, 8}, false)
	bridge := sw.NewBridgeNode()
	require.NoError(t, bridge.Start(ctx))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	fullCfg := sw.DefaultTestConfig(node.Full)
	fullCfg.Header.TrustedPeers = append(fullCfg.Header.TrustedPeers, addrs[0].String())
	fullNode := sw.NewNodeWithConfig(node.Full, fullCfg)
	require.NoError(t, fullNode.Start(ctx))

	addrsFull, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(fullNode.Host))
	require.NoError(t, err)

	lightCfg := sw.DefaultTestConfig(node.Light)
	lightCfg.Header.TrustedPeers = append(lightCfg.Header.TrustedPeers, addrsFull[0].String())
	lightNode := sw.NewNodeWithConfig(node.Light, lightCfg)
	require.NoError(t, lightNode.Start(ctx))

	height, err := fullNode.BlobServ.Submit(ctx, blobs...)
	require.NoError(t, err)
	_, err = fullNode.HeaderServ.WaitForHeight(ctx, height)
	require.NoError(t, err)

	_, err = lightNode.HeaderServ.WaitForHeight(ctx, height)
	require.NoError(t, err)

	newBlobs, err := fullNode.BlobServ.GetAll(ctx, height, blobs[0].NamespaceID(), blobs[1].NamespaceID())
	require.NoError(t, err)
	require.Len(t, newBlobs, 2)
	require.True(t, bytes.Equal(blobs[0].Commitment(), newBlobs[0].Commitment()))
	require.True(t, bytes.Equal(blobs[1].Commitment(), newBlobs[1].Commitment()))
}

func TestBlobModuleNotFound(t *testing.T) {
	const (
		btime = time.Millisecond * 300
	)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)
	blobs := blob.GenerateBlobs(t, []int{8, 8}, false)
	bridge := sw.NewBridgeNode()
	require.NoError(t, bridge.Start(ctx))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	fullCfg := sw.DefaultTestConfig(node.Full)
	fullCfg.Header.TrustedPeers = append(fullCfg.Header.TrustedPeers, addrs[0].String())
	fullNode := sw.NewNodeWithConfig(node.Full, fullCfg)
	require.NoError(t, fullNode.Start(ctx))

	addrsFull, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(fullNode.Host))
	require.NoError(t, err)

	lightCfg := sw.DefaultTestConfig(node.Light)
	lightCfg.Header.TrustedPeers = append(lightCfg.Header.TrustedPeers, addrsFull[0].String())
	lightNode := sw.NewNodeWithConfig(node.Light, lightCfg)
	require.NoError(t, lightNode.Start(ctx))

	height, err := fullNode.BlobServ.Submit(ctx, blobs[0])
	require.NoError(t, err)
	_, err = fullNode.HeaderServ.WaitForHeight(ctx, height)
	require.NoError(t, err)

	_, err = lightNode.HeaderServ.WaitForHeight(ctx, height)
	require.NoError(t, err)

	b, err := fullNode.BlobServ.Get(ctx, height, blobs[1].NamespaceID(), blobs[1].Commitment())
	assert.Nil(t, b)
	require.Error(t, err)
	require.ErrorIs(t, err, blob.ErrBlobNotFound)
}
