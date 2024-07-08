package testnet

import (
	"fmt"

	"github.com/celestiaorg/celestia-app/v2/test/e2e/testnet"
	"github.com/celestiaorg/celestia-app/v2/test/util/genesis" // Updated this import
	"github.com/celestiaorg/knuu/pkg/knuu"
	"github.com/rs/zerolog/log"
)

// LocalTestnet extends the testnet from celestia-app
type NodeTestnet struct {
	testnet.Testnet
	executor *knuu.Executor
	nodes    []*Node
}

// NewLocalTestnet creates a new instance of LocalTestnet
func NewNodeTestnet(name string, seed int64, grafana *testnet.GrafanaInfo, chainID string, genesisModifier ...genesis.Modifier) (*NodeTestnet, error) {
	tn, err := testnet.New(name, seed, grafana, chainID, genesisModifier...)
	if err != nil {
		return nil, err
	}
	executor, err := knuu.NewExecutor()
	if err != nil {
		return nil, err
	}
	return &NodeTestnet{*tn, executor, []*Node{}}, nil
}

// DaNodes returns all DA nodes
func (nt *NodeTestnet) DaNodes() []*Node {
	return nt.nodes
}

// NodeCleanup cleans up the nodes
func (nt *NodeTestnet) NodeCleanup() {
	for _, node := range nt.nodes {
		err := node.Instance.Destroy()
		if err != nil {
			log.Error().Err(err).Msg("error destroying node")
		}
	}
	err := nt.executor.Destroy()
	if err != nil {
		log.Error().Err(err).Msg("error destroying executor")
	}
}

// CreateBridgeNode creates a new bridge node
func (nt *NodeTestnet) CreateAndStartBridgeNode(name string, version string, resources testnet.Resources) error {
	bridge, err := CreateAndStartBridge(nt.executor, name, version, nt.Node(0).Instance, resources)
	if err != nil {
		return err
	}
	nt.nodes = append(nt.nodes, bridge)
	return nil
}

// CreateBridgeNodes creates a new bridge nodes
func (nt *NodeTestnet) CreateAndStartBridgeNodes(count int, name string, version string, resources testnet.Resources) error {
	for i := 0; i < count; i++ {
		err := nt.CreateAndStartBridgeNode(fmt.Sprintf("%s-%d", name, i), version, resources)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateFullNode creates a new full node
func (nt *NodeTestnet) CreateAndStartFullNode(name string, version string, resources testnet.Resources) error {
	full, err := CreateAndStartNode(nt.executor, name, version, "full", nt.Node(0).Instance, nt.nodes[0], resources)
	if err != nil {
		return err
	}
	nt.nodes = append(nt.nodes, full)
	return nil
}

// CreateFullNodes creates a new full nodes
func (nt *NodeTestnet) CreateAndStartFullNodes(count int, name string, version string, resources testnet.Resources) error {
	for i := 0; i < count; i++ {
		err := nt.CreateAndStartFullNode(fmt.Sprintf("%s-%d", name, i), version, resources)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateLightNode creates a new light node
func (nt *NodeTestnet) CreateAndStartLightNode(name string, version string, resources testnet.Resources) error {
	light, err := CreateAndStartNode(nt.executor, name, version, "light", nt.Node(0).Instance, nt.nodes[0], resources)
	if err != nil {
		return err
	}
	nt.nodes = append(nt.nodes, light)
	return nil
}

// CreateLightNodes creates a new light nodes
func (nt *NodeTestnet) CreateAndStartLightNodes(count int, name string, version string, resources testnet.Resources) error {
	for i := 0; i < count; i++ {
		err := nt.CreateAndStartLightNode(fmt.Sprintf("%s-%d", name, i), version, resources)
		if err != nil {
			return err
		}
	}
	return nil
}
