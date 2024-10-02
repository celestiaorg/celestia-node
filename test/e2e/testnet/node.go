package testnet

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/knuu/pkg/instance"
	"github.com/celestiaorg/knuu/pkg/sidecars/observability"
	"github.com/prometheus/common/expfmt"
)

const (
	rpcPort           = 26658
	p2pPort           = 2121
	prometheusPort    = 9090
	otlpRemotePort    = 4318
	dockerSrcURL      = "ghcr.io/celestiaorg/celestia-node"
	remoteRootDir     = "/home/celestia"
	txsimRootDir      = "/home/celestia"
	celestiaCustomEnv = "CELESTIA_CUSTOM"
)

type Node struct {
	Name     string
	Type     node.Type
	Version  string
	Instance *instance.Instance

	rpcProxyHost        string
	prometheusProxyHost string
}

type JSONRPCError struct {
	Code    int
	Message string
	Data    string
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSONRPC Error - Code: %d, Message: %s, Data: %s", e.Code, e.Message, e.Data)
}

func (nt *NodeTestnet) initInstance(ctx context.Context, opts InstanceOptions) (*instance.Instance, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	ins, err := nt.Knuu().NewInstance(opts.InstanceName)
	if err != nil {
		return nil, err
	}

	if err := ins.Build().SetImage(ctx, fmt.Sprintf("%s:%s", dockerSrcURL, opts.Version)); err != nil {
		return nil, err
	}

	for _, port := range []int{p2pPort, rpcPort} {
		if err := ins.Network().AddPortTCP(port); err != nil {
			return nil, err
		}
	}

	err = ins.Build().ExecuteCommand("celestia", strings.ToLower(opts.NodeType.String()), "init", "--node.store", remoteRootDir)
	if err != nil {
		return nil, err
	}

	if err := ins.Build().Commit(ctx); err != nil {
		return nil, err
	}

	chainID, err := opts.ChainID(ctx)
	if err != nil {
		return nil, err
	}

	genesisHash, err := opts.GenesisHash(ctx)
	if err != nil {
		return nil, err
	}

	err = ins.Build().SetEnvironmentVariable(celestiaCustomEnv, fmt.Sprintf("%s:%s", chainID, genesisHash))
	if err != nil {
		return nil, err
	}

	obsy := observability.New()
	if err := obsy.SetOtelEndpoint(otlpRemotePort); err != nil {
		return nil, err
	}

	if err := obsy.SetPrometheusExporter(fmt.Sprintf("0.0.0.0:%d", prometheusPort)); err != nil {
		return nil, err
	}

	if err := ins.Sidecars().Add(ctx, obsy); err != nil {
		return nil, err
	}

	return ins, nil
}

func (nt *NodeTestnet) CreateNode(ctx context.Context, opts InstanceOptions, trustedNode *Node) (*Node, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	nodeInst, err := nt.initInstance(ctx, opts)
	if err != nil {
		return nil, err
	}

	//TODO: implement an IsEmpty method for Resources in the app testnet pkg
	if opts.Resources == (testnet.Resources{}) {
		opts.Resources = DefaultBridgeResources
	}

	err = nodeInst.Resources().SetMemory(opts.Resources.MemoryRequest, opts.Resources.MemoryLimit)
	if err != nil {
		return nil, err
	}

	if err := nodeInst.Resources().SetCPU(opts.Resources.CPU); err != nil {
		return nil, err
	}

	startCmd := []string{
		"celestia",
		strings.ToLower(opts.NodeType.String()),
		"start",
		"--node.store", remoteRootDir,
		"--metrics",
		"--metrics.endpoint", fmt.Sprintf("localhost:%d", otlpRemotePort),
		"--metrics.tls=false",
	}

	if opts.NodeType == node.Bridge {
		consensusIP, err := opts.consensus.Network().GetIP(ctx)
		if err != nil {
			return nil, err
		}
		startCmd = append(startCmd, "--core.ip", consensusIP, "--rpc.addr", "0.0.0.0")

	} else {
		trustedPeers, err := getTrustedPeers(ctx, trustedNode)
		if err != nil {
			return nil, err
		}
		startCmd = append(startCmd, "--headers.trusted-peers", trustedPeers)
	}

	return &Node{
		Name:     opts.InstanceName,
		Type:     opts.NodeType,
		Version:  opts.Version,
		Instance: nodeInst,
	}, nil
}

func (nt *NodeTestnet) CreateAndStartNode(ctx context.Context, opts InstanceOptions, trustedNode *Node) (*Node, error) {
	node, err := nt.CreateNode(ctx, opts, trustedNode)
	if err != nil {
		return nil, ErrFailedToCreateNode.Wrap(err)
	}

	if err := node.Instance.Execution().Start(ctx); err != nil {
		return nil, err
	}

	rpcProxyHost, err := node.Instance.Network().AddHost(ctx, rpcPort)
	if err != nil {
		return nil, err
	}
	prometheusProxyHost, err := node.Instance.Network().AddHost(ctx, prometheusPort)
	if err != nil {
		return nil, err
	}

	node.rpcProxyHost = rpcProxyHost
	node.prometheusProxyHost = prometheusProxyHost

	return node, nil
}

// GetMetric returns a metric from the node
func (n *Node) GetMetric(metricName string) (float64, error) {
	host := n.AddressPrometheus()

	resp, err := http.Get(fmt.Sprintf("%s/metrics", host))
	if err != nil {
		return 0, ErrFailedToFetchPrometheusData.Wrap(err)
	}
	defer resp.Body.Close()

	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return 0, ErrFailedToParsePrometheusMetrics.Wrap(err)
	}

	var metricValue float64
	found := false

	if metricFamily, ok := metricFamilies[metricName]; ok {
		for _, metric := range metricFamily.GetMetric() {
			switch {
			case metric.Counter != nil:
				metricValue = metric.Counter.GetValue()
				found = true
			case metric.Gauge != nil:
				metricValue = metric.Gauge.GetValue()
				found = true
			case metric.Untyped != nil:
				metricValue = metric.Untyped.GetValue()
				found = true
			case metric.Summary != nil:
				metricValue = metric.Summary.GetSampleSum()
				found = true
			case metric.Histogram != nil:
				metricValue = metric.Histogram.GetSampleSum()
				found = true
			}
		}
	}

	if !found {
		return 0, ErrMetricNotFound.WithParams(metricName)
	}

	return metricValue, nil
}

// AddressRPC returns an RPC endpoint address for the node.
// This returns the proxy host that can be used to communicate with the node
func (n Node) AddressRPC() string {
	return n.rpcProxyHost
}

// AddressGRPC returns the GRPC endpoint address for the node.
func (n Node) AddressPrometheus() string {
	return n.prometheusProxyHost
}
