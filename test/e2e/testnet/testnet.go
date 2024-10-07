package testnet

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"github.com/celestiaorg/celestia-app/v3/test/util/genesis"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/knuu/pkg/instance"
)

// LocalTestnet extends the testnet from celestia-app
type NodeTestnet struct {
	testnet.Testnet
	executor *instance.Instance
	nodes    []*Node
}

type NodeTestnetOptions struct {
	Name            string
	Seed            int64
	Grafana         *testnet.GrafanaInfo
	ChainID         string
	GenesisModifier []genesis.Modifier
}

// NewLocalTestnet creates a new instance of LocalTestnet
func NewNodeTestnet(ctx context.Context, opts NodeTestnetOptions) (*NodeTestnet, error) {
	tn, err := testnet.New(ctx, opts.Name, opts.Seed, opts.Grafana, opts.ChainID, opts.GenesisModifier...)
	if err != nil {
		return nil, err
	}

	ex := Executor{Kn: tn.Knuu()}
	executorInstance, err := ex.NewInstance(ctx, "executor")
	if err != nil {
		return nil, err
	}
	return &NodeTestnet{*tn, executorInstance, []*Node{}}, nil
}

// DaNodes returns all DA nodes
func (nt *NodeTestnet) DaNodes() []*Node {
	return nt.nodes
}

// NodeCleanup cleans up the nodes
func (nt *NodeTestnet) NodeCleanup(ctx context.Context) {
	nt.Cleanup(ctx)
}

// CreateBridgeNode creates a new bridge node
func (nt *NodeTestnet) CreateAndStartBridgeNode(ctx context.Context, opts InstanceOptions) error {
	opts.NodeType = node.Bridge
	bridge, err := nt.CreateAndStartNode(ctx, opts, nil)
	if err != nil {
		return err
	}
	nt.nodes = append(nt.nodes, bridge)
	return nil
}

// CreateBridgeNodes creates a new bridge nodes
func (nt *NodeTestnet) CreateAndStartBridgeNodes(ctx context.Context, count int, opts InstanceOptions) error {
	for i := 0; i < count; i++ {
		err := nt.CreateAndStartBridgeNode(ctx, InstanceOptions{
			InstanceName: fmt.Sprintf("%s-%d", opts.InstanceName, i),
			Version:      opts.Version,
			Resources:    opts.Resources,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateFullNode creates a new full node
func (nt *NodeTestnet) CreateAndStartFullNode(ctx context.Context, opts InstanceOptions) error {
	opts.NodeType = node.Full
	full, err := nt.CreateAndStartNode(ctx, opts, nt.nodes[0])
	if err != nil {
		return err
	}
	nt.nodes = append(nt.nodes, full)
	return nil
}

// CreateFullNodes creates a new full nodes
func (nt *NodeTestnet) CreateAndStartFullNodes(ctx context.Context, count int, opts InstanceOptions) error {
	for i := 0; i < count; i++ {
		err := nt.CreateAndStartFullNode(ctx, InstanceOptions{
			InstanceName: fmt.Sprintf("%s-%d", opts.InstanceName, i),
			Version:      opts.Version,
			Resources:    opts.Resources,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateLightNode creates a new light node
func (nt *NodeTestnet) CreateAndStartLightNode(ctx context.Context, opts InstanceOptions) error {
	opts.NodeType = node.Light
	light, err := nt.CreateAndStartNode(ctx, opts, nt.nodes[0])
	if err != nil {
		return err
	}
	nt.nodes = append(nt.nodes, light)
	return nil
}

// CreateLightNodes creates a new light nodes
func (nt *NodeTestnet) CreateAndStartLightNodes(ctx context.Context, count int, opts InstanceOptions) error {
	for i := 0; i < count; i++ {
		err := nt.CreateAndStartLightNode(ctx, InstanceOptions{
			InstanceName: fmt.Sprintf("%s-%d", opts.InstanceName, i),
			Version:      opts.Version,
			Resources:    opts.Resources,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
