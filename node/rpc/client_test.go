package rpc

import (
	"context"
	"testing"

	"github.com/celestiaorg/celestia-core/abci/example/kvstore"
	"github.com/celestiaorg/celestia-core/node"
	rpctest "github.com/celestiaorg/celestia-core/rpc/test"
)

func TestNewClient(t *testing.T) {
	_, backgroundNode := newClient(t)
	//nolint:errcheck
	backgroundNode.Stop()
}

func TestClient_GetStatus(t *testing.T) {
	client, backgroundNode := newClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	//nolint:errcheck
	t.Cleanup(func() {
		backgroundNode.Stop()
		cancel()
	})

	status, err := client.GetStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(status.NodeInfo)
}

func TestClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	client, backgroundNode := newClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	//nolint:errcheck
	t.Cleanup(func() {
		backgroundNode.Stop()
		cancel()
	})

	// make 3 blocks
	if err := client.Start(); err != nil {
		t.Fatal(err)
	}
	eventChan, err := client.StartBlockSubscription(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		<-eventChan
	}
	if err := client.StopBlockSubscription(ctx); err != nil {
		t.Fatal(err)
	}

	height := int64(2)
	block, err := client.GetBlock(ctx, &height)
	if err != nil {
		t.Fatal(err)
	}

	if block.Block.Height != height {
		t.Fatalf("mismatched block heights: expected %v, got %v", height, block.Block.Height)
	}
}

func newClient(t *testing.T) (*Client, *node.Node) {
	backgroundNode := startCoreNode()

	endpoint := backgroundNode.Config().RPC.ListenAddress
	// separate the protocol from the endpoint
	protocol, ip := endpoint[:3], endpoint[6:]

	client, err := NewClient(protocol, ip)
	if err != nil {
		t.Fatal(err)
	}
	return client, backgroundNode
}

func startCoreNode() *node.Node {
	app := kvstore.NewApplication()
	app.RetainBlocks = 10
	return rpctest.StartTendermint(app, rpctest.SuppressStdout)
}
