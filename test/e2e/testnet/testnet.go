package testnet

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"

	nodebuilderNode "github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/test/e2e/prometheus"
	"github.com/celestiaorg/knuu/pkg/instance"
	"github.com/celestiaorg/knuu/pkg/knuu"

	"github.com/rs/zerolog/log"
)

// LocalTestnet extends the testnet from celestia-app
type NodeTestnet struct {
	testnet.Testnet
	knuu  *knuu.Knuu
	nodes []*Node

	Prometheus *prometheus.Prometheus
}

// NewLocalTestnet creates a new instance of LocalTestnet
func NewNodeTestnet(ctx context.Context, kn *knuu.Knuu, opts testnet.Options) (*NodeTestnet, error) {
	tn, err := testnet.New(kn, opts)
	if err != nil {
		return nil, err
	}

	prom, err := prometheus.New(kn)
	if err != nil {
		return nil, err
	}

	return &NodeTestnet{
		Testnet:    *tn,
		nodes:      []*Node{},
		knuu:       kn,
		Prometheus: prom,
	}, nil
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
			return nil
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
	log.Info().Msgf("Creating %s node", nodeType)
	node, err := NewNode(ctx, fmt.Sprintf("%s%d", nodeType, len(nt.nodes)), version, nodeType, chainID, genesisHash, coreIP, bootstrapper, archival, resources, nt.knuu)
	if err != nil {
		return err
	}
	nt.nodes = append(nt.nodes, node)
	return nil
}

func (nt *NodeTestnet) SetupDA(ctx context.Context) error {
	// TODO: add the nodes as a peer to each other
	for _, node := range nt.nodes {
		log.Info().Msgf("Initializing %s node", node.Type)
		trustedPeers := []string{}
		for _, otherNode := range nt.nodes {
			if otherNode.Name != node.Name {
				trustedPeers = append(trustedPeers, otherNode.Name)
			}
		}
		if err := node.Init(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (nt *NodeTestnet) WaitToSyncDANodes(ctx context.Context) error {
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
	log.Info().Msg("waiting for DA nodes to sync")
	err = nt.WaitToSync(ctx)
	if err != nil {
		return err
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

// NodeCleanup cleans up the nodes
func (nt *NodeTestnet) NodeCleanup(ctx context.Context) {
	nt.Cleanup(ctx)
}
