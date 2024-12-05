package testnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog/log"

	"github.com/celestiaorg/celestia-app/v3/app"
	"github.com/celestiaorg/celestia-app/v3/app/encoding"
	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	nodebuilderNode "github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	e2ePrometheus "github.com/celestiaorg/celestia-node/test/e2e/prometheus"
	"github.com/celestiaorg/knuu/pkg/instance"
	"github.com/celestiaorg/knuu/pkg/knuu"
	"github.com/celestiaorg/knuu/pkg/sidecars/observability"
	"github.com/libp2p/go-libp2p/core/host"
)

type Node struct {
	Name         string
	Type         nodebuilderNode.Type
	Version      string
	Instance     *instance.Instance
	sidecars     []instance.SidecarManager
	ChainID      string
	GenesisHash  string
	CoreIP       string
	nodeID       string
	bootstrapper bool
	archival     bool
	rpcProxyHost string
}

type JSONRPCError struct {
	Code    int
	Message string
	Data    string
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSONRPC Error - Code: %d, Message: %s, Data: %s", e.Code, e.Message, e.Data)
}

func NewNode(ctx context.Context,
	name string,
	version string,
	nodeType nodebuilderNode.Type,
	chainID string,
	genesisHash string,
	coreIP string,
	bootstrapper bool,
	archival bool,
	resources testnet.Resources,
	kn *knuu.Knuu,
) (*Node, error) {
	// only type bridge and full can be bootstrapper
	if bootstrapper && nodeType != node.Bridge && nodeType != node.Full {
		return nil, fmt.Errorf("bootstrapper can only be true for bridge and full nodes")
	}
	knInstance, err := kn.NewInstance(name)
	if err != nil {
		return nil, err
	}
	err = knInstance.Build().SetImage(ctx, DockerImageName(version))
	if err != nil {
		return nil, err
	}

	for _, port := range []int{p2pPortTcp, websocketPort, restPort, rpcPort} {
		if err := knInstance.Network().AddPortTCP(port); err != nil {
			return nil, err
		}
	}
	for _, port := range []int{p2pPortUdp} {
		if err := knInstance.Network().AddPortUDP(port); err != nil {
			return nil, err
		}
	}
	// TODO: add obsy sidecar
	err = knInstance.Resources().SetMemory(resources.MemoryRequest, resources.MemoryLimit)
	if err != nil {
		return nil, err
	}
	err = knInstance.Resources().SetCPU(resources.CPU)
	if err != nil {
		return nil, err
	}
	err = knInstance.Storage().AddVolumeWithOwner(remoteRootDir, resources.Volume, 10001)
	if err != nil {
		return nil, err
	}

	return &Node{
		Name:         name,
		Type:         nodeType,
		Version:      version,
		Instance:     knInstance,
		ChainID:      chainID,
		GenesisHash:  genesisHash,
		CoreIP:       coreIP,
		bootstrapper: bootstrapper,
		archival:     archival,
	}, nil
}

func (n *Node) Init(ctx context.Context, prometheus *e2ePrometheus.Prometheus) error {
	tmpFolder, err := os.MkdirTemp("", "e2e_test_")
	if err != nil {
		return fmt.Errorf("creating temporary folder: %w", err)
	}
	defer os.RemoveAll(tmpFolder)

	nodeConfig := nodebuilder.DefaultConfig(n.Type)
	nodeConfig.Core.IP = n.CoreIP
	nodeConfig.RPC.Address = "0.0.0.0"
	err = nodebuilder.Init(*nodeConfig, tmpFolder, n.Type)
	if err != nil {
		return fmt.Errorf("initializing node: %w", err)
	}
	encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	keysPath := filepath.Join(tmpFolder, "keys")
	ring, err := keyring.New(app.Name, nodeConfig.State.DefaultBackendName, keysPath, os.Stdin, encConf.Codec)
	if err != nil {
		return fmt.Errorf("creating keyring: %w", err)
	}
	store, err := nodebuilder.OpenStore(tmpFolder, ring)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	node, err := nodebuilder.NewWithConfig(n.Type, p2p.Mainnet, store, nodeConfig)
	if err != nil {
		return fmt.Errorf("creating node: %w", err)
	}
	err = os.RemoveAll(filepath.Join(tmpFolder, "blocks"))
	if err != nil {
		return fmt.Errorf("removing blocks folder: %w", err)
	}
	err = os.RemoveAll(filepath.Join(tmpFolder, "data"))
	if err != nil {
		return fmt.Errorf("removing data folder: %w", err)
	}
	err = os.MkdirAll(filepath.Join(tmpFolder, "data"), 0755)
	if err != nil {
		return fmt.Errorf("creating data folder: %w", err)
	}
	n.nodeID = peer.AddrInfosToIDs([]peer.AddrInfo{*host.InfoFromHost(node.Host)})[0].String()
	log.Info().Msgf("id: %+v", n.nodeID)
	err = os.WriteFile(filepath.Join(tmpFolder, "data", ".folderkeep"), []byte("This file is needed so that knuu can start the node"), 0644)
	if err != nil {
		return fmt.Errorf("writing placeholder file: %w", err)
	}

	if prometheus != nil {
		obsy, err := createObservability()
		if err != nil {
			return err
		}

		if err := n.Instance.Sidecars().Add(ctx, obsy); err != nil {
			return err
		}

		// Expose the prometheus exporter on the otel collector instance
		if err := obsy.Instance().Network().AddPortTCP(prometheusExporterPort); err != nil {
			return err
		}

		err = prometheus.AddScrapeJob(ctx, e2ePrometheus.ScrapeJob{
			Name:     n.Name,
			Target:   fmt.Sprintf("%s:%d", n.Name, prometheusExporterPort),
			Interval: prometheusScrapeInterval,
		})
		if err != nil {
			return err
		}
	}

	err = n.Instance.Build().Commit(ctx)
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	for _, sc := range n.sidecars {
		if err := n.Instance.Sidecars().Add(ctx, sc); err != nil {
			return fmt.Errorf("adding sidecar: %w", err)
		}
	}

	if err := n.Instance.Storage().AddFolder(tmpFolder, "/home/celestia", "10001:10001"); err != nil {
		return fmt.Errorf("adding folder: %w", err)
	}
	if n.bootstrapper {
		if err := n.Instance.Build().SetEnvironmentVariable(celestiaBootstrapEnv, fmt.Sprintf("%t", n.bootstrapper)); err != nil {
			return fmt.Errorf("setting bootstrap env: %w", err)
		}
	}

	args := fmt.Sprintf("celestia %s start --node.store %s", strings.ToLower(n.Type.String()), remoteRootDir)
	if n.archival {
		args = fmt.Sprintf("%s --archival", args)
	}
	if prometheus != nil {
		args = fmt.Sprintf("%s --metrics --metrics.endpoint 0.0.0.0:%d --metrics.tls=false", args, otlpPort)
	}
	if err := n.Instance.Build().SetStartCommand("bash", "-c"); err != nil {
		return fmt.Errorf("setting start command: %w", err)
	}
	if err := n.Instance.Build().SetArgs(args); err != nil {
		return fmt.Errorf("setting args: %w", err)
	}

	return nil
}

func (n *Node) GetNodeID(ctx context.Context) (string, error) {
	return n.nodeID, nil
}

func (n *Node) AddressP2P(ctx context.Context) (string, error) {
	hostName := n.Instance.Network().HostName()
	return fmt.Sprintf("/dns/%s/tcp/2121/p2p/%s", hostName, n.nodeID), nil
}

func createObservability() (*observability.Obsy, error) {
	obsy := observability.New()
	if err := obsy.SetOtelEndpoint(otlpPort); err != nil {
		return nil, err
	}
	if err := obsy.SetPrometheusEndpoint(prometheusPort, "libp2p", "10s"); err != nil {
		return nil, err
	}

	if err := obsy.SetPrometheusExporter(fmt.Sprintf("0.0.0.0:%d", prometheusExporterPort)); err != nil {
		return nil, err
	}

	return obsy, nil
}

func (n *Node) Start(ctx context.Context, bootstrappers []string) error {
	if err := n.StartAsync(ctx, bootstrappers); err != nil {
		return fmt.Errorf("starting async: %w", err)
	}

	return n.WaitUntilStarted(ctx)
}

func (n *Node) StartAsync(ctx context.Context, bootstrappers []string) error {
	envValue := fmt.Sprintf("%s:%s", n.ChainID, n.GenesisHash)
	if len(bootstrappers) > 0 {
		envValue = fmt.Sprintf("%s:%s", envValue, strings.Join(bootstrappers, ","))
	}
	if err := n.Instance.Build().SetEnvironmentVariable(celestiaCustomEnv, envValue); err != nil {
		return fmt.Errorf("setting celestia custom env: %w", err)
	}

	return n.Instance.Execution().StartAsync(ctx)
}

func (n *Node) WaitUntilStarted(ctx context.Context) error {
	if err := n.Instance.Execution().WaitInstanceIsRunning(ctx); err != nil {
		return fmt.Errorf("waiting for instance to start: %w", err)
	}

	return nil
}

// AddressRPC returns an RPC endpoint address for the node.
// This returns the proxy host that can be used to communicate with the node
func (n Node) AddressRPC() string {
	return n.rpcProxyHost
}

func DockerImageName(version string) string {
	return fmt.Sprintf("%s:%s", dockerSrcURL, version)
}
