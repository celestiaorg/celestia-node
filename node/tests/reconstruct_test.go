package tests

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/tests/swamp"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getMultiAddr(h host.Host) string {
	addrs, _ := peer.AddrInfoToP2pAddrs(host.InfoFromHost(h))
	// require.NoError(t, err)
	return addrs[0].String()
}

func RandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func TestFullReconstructFromBridge(t *testing.T) {

	sw := swamp.NewSwamp(t, swamp.DefaultComponents())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	ticker := time.NewTicker(10 * time.Millisecond)
	done := make(chan bool)

	go func(tt *testing.T) {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, e := sw.CreateTx(ctx, "kuna="+RandomString(8))
				require.NoError(tt, e)
			}
		}
	}(t)

	bridge := sw.NewBridgeNode()

	sw.WaitTillHeight(ctx, 20)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	full := sw.NewFullNode(node.WithTrustedPeers(addrs[0].String()))

	require.NoError(t, full.Start(ctx))

	h, err = full.HeaderServ.GetByHeight(ctx, 10)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 10))

	require.NoError(t, full.ShareServ.SharesAvailable(ctx, h.DAH))

	time.Sleep(1600 * time.Millisecond)
	ticker.Stop()
	done <- true
	fmt.Println("Ticker stopped")
}

func TestFullReconstructFromLights(t *testing.T) {

	sw := swamp.NewSwamp(t, swamp.DefaultComponents())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	ticker := time.NewTicker(10 * time.Millisecond)
	done := make(chan bool)

	go func(tt *testing.T) {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, e := sw.CreateTx(ctx, "kuna="+RandomString(8))
				require.NoError(tt, e)
			}
		}
	}(t)

	bridge := sw.NewBridgeNode()

	sw.WaitTillHeight(ctx, 20)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	var trustedPeers []string
	for i := 0; i < 10; i++ {
		light := sw.NewLightNode(node.WithTrustedPeers(addrs[0].String()))
		err = light.Start(ctx)
		require.NoError(t, err)
		trustedPeers = []string{getMultiAddr(light.Host)}
	}

	full := sw.NewFullNode(node.WithTrustedPeers(trustedPeers...))
	err = sw.Network.UnlinkPeers(bridge.Host.ID(), full.Host.ID())
	require.NoError(t, err)

	require.NoError(t, full.Start(ctx))

	h, err = full.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	require.NoError(t, full.ShareServ.SharesAvailable(ctx, h.DAH))

	time.Sleep(1600 * time.Millisecond)
	ticker.Stop()
	done <- true
	fmt.Println("Ticker stopped")
}
