package testnet

import (
	"context"
	"fmt"
	"log"

	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"

	nodebuilderNode "github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/test/e2e/prometheus"
	"github.com/celestiaorg/knuu/pkg/instance"
	"github.com/celestiaorg/knuu/pkg/knuu"
)

// LocalTestnet extends the testnet from celestia-app
type NodeTestnet struct {
	testnet.Testnet
	knuu  *knuu.Knuu
	nodes []*Node

	Prometheus *prometheus.Prometheus

	logger *log.Logger
}

// NewLocalTestnet creates a new instance of LocalTestnet
func NewNodeTestnet(ctx context.Context, logger *log.Logger, kn *knuu.Knuu, opts testnet.Options) (*NodeTestnet, error) {
	tn, err := testnet.New(logger, kn, opts)
	if err != nil {
		return nil, err
	}

	return &NodeTestnet{
		Testnet: *tn,
		nodes:   []*Node{},
		knuu:    kn,
		logger:  logger,
	}, nil
}

func (nt *NodeTestnet) EnablePrometheus(ctx context.Context) error {
	prom, err := prometheus.New(nt.knuu)
	if err != nil {
		return err
	}
	nt.Prometheus = prom
	return nil
}

func (nt *NodeTestnet) CreateBridgeNode(ctx context.Context, version string, chainID string, genesisHash string, coreIP string, bootstrapper bool, archival bool, resources testnet.Resources) error {
	err := nt.CreateDANode(ctx, version, nodebuilderNode.Bridge, chainID, genesisHash, coreIP, bootstrapper, archival, resources)
	if err != nil {
		return err
	}
	return nil
}

func (nt *NodeTestnet) CreateBridgeNodes(ctx context.Context, count int, version string, chainID string, genesisHash string, coreIP string, bootstrapper bool, archival bool, resources testnet.Resources) error {
	for i := 0; i < count; i++ {
		err := nt.CreateBridgeNode(ctx, version, chainID, genesisHash, coreIP, bootstrapper, archival, resources)
		if err != nil {
			return err
		}
	}
	return nil
}

func (nt *NodeTestnet) CreateFullNode(ctx context.Context, version string, chainID string, genesisHash string, coreIP string, bootstrapper bool, archival bool, resources testnet.Resources) error {
	err := nt.CreateDANode(ctx, version, nodebuilderNode.Full, chainID, genesisHash, coreIP, bootstrapper, archival, resources)
	if err != nil {
		return err
	}
	return nil
}

func (nt *NodeTestnet) CreateFullNodes(ctx context.Context, count int, version string, chainID string, genesisHash string, coreIP string, bootstrapper bool, archival bool, resources testnet.Resources) error {
	for i := 0; i < count; i++ {
		err := nt.CreateFullNode(ctx, version, chainID, genesisHash, coreIP, bootstrapper, archival, resources)
		if err != nil {
			return err
		}
	}
	return nil
}

func (nt *NodeTestnet) CreateLightNode(ctx context.Context, version string, chainID string, genesisHash string, coreIP string, resources testnet.Resources) error {
	err := nt.CreateDANode(ctx, version, nodebuilderNode.Light, chainID, genesisHash, coreIP, false, false, resources)
	if err != nil {
		return err
	}
	return nil
}

func (nt *NodeTestnet) CreateLightNodes(ctx context.Context, count int, version string, chainID string, genesisHash string, coreIP string, resources testnet.Resources) error {
	for i := 0; i < count; i++ {
		err := nt.CreateLightNode(ctx, version, chainID, genesisHash, coreIP, resources)
		if err != nil {
			return err
		}
	}
	return nil
}

func (nt *NodeTestnet) CreateDANode(
	ctx context.Context,
	version string,
	nodeType nodebuilderNode.Type,
	chainID string,
	genesisHash string,
	coreIP string,
	bootstrapper bool,
	archival bool,
	resources testnet.Resources,
) error {
	nt.logger.Printf("Creating %s node", nodeType)
	node, err := NewNode(ctx, nt.logger, fmt.Sprintf("%s%d", nodeType, len(nt.nodes)), version, nodeType, chainID, genesisHash, coreIP, bootstrapper, archival, resources, nt.knuu)
	if err != nil {
		return err
	}
	nt.nodes = append(nt.nodes, node)
	return nil
}

func (nt *NodeTestnet) SetupDA(ctx context.Context) error {
	for _, node := range nt.nodes {
		nt.logger.Printf("Initializing %s node", node.Type)
		trustedPeers := []string{}
		for _, otherNode := range nt.nodes {
			if otherNode.Name != node.Name {
				trustedPeers = append(trustedPeers, otherNode.Name)
			}
		}
		if err := node.Init(ctx, nt.Prometheus); err != nil {
			return err
		}
	}
	return nil
}

func (nt *NodeTestnet) WaitToSyncDANodes(ctx context.Context) error {
	// TODO: not needed right now, but maybe later
	return nil
}

func (nt *NodeTestnet) StartDANodes(ctx context.Context) error {
	bootstrapperNodes := []*Node{}
	otherNodes := []*Node{}
	bootstrapperAddresses := []string{}
	for _, node := range nt.nodes {
		if node.bootstrapper {
			bootstrapperNodes = append(bootstrapperNodes, node)
			bootstrapperAddress, err := node.AddressP2P(ctx)
			if err != nil {
				return err
			}
			bootstrapperAddresses = append(bootstrapperAddresses, bootstrapperAddress)
		} else {
			otherNodes = append(otherNodes, node)
		}
	}

	for _, node := range bootstrapperNodes {
		if err := node.StartAsync(ctx, bootstrapperAddresses); err != nil {
			return err
		}
	}
	for _, node := range bootstrapperNodes {
		if err := node.WaitUntilStarted(ctx); err != nil {
			return err
		}
	}
	for _, node := range otherNodes {
		if err := node.StartAsync(ctx, bootstrapperAddresses); err != nil {
			return err
		}
	}
	for _, node := range otherNodes {
		if err := node.WaitUntilStarted(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (nt *NodeTestnet) StartDA(ctx context.Context) error {
	err := nt.StartDANodes(ctx)
	if err != nil {
		return err
	}
	// wait for nodes to sync
	nt.logger.Printf("waiting for DA nodes to sync")
	err = nt.WaitToSync(ctx)
	if err != nil {
		return err
	}
	if nt.Prometheus != nil {
		err = nt.Prometheus.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start prometheus: %w", err)
		}
	}
	return nil
}

// DaNodes returns all DA nodes
func (nt *NodeTestnet) DaNodes() []*Node {
	return nt.nodes
}

func (nt *NodeTestnet) NewInstance(name string) (*instance.Instance, error) {
	return nt.knuu.NewInstance(name)
}
