package share

import (
	"context"
	"os"
	"strconv"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	p2pDisc "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/celestiaorg/celestia-node/share"
)

// RDAModule provides RDA grid functionality.
type RDAModule struct {
	Service *share.RDANodeService
	API     share.RDAAPI
}

// newRDAService creates and initializes the RDA node service.
// disc is the libp2p RoutingDiscovery (backed by DHT) for grid peer discovery.
func newRDAService(
	ctx context.Context,
	h host.Host,
	ps *pubsub.PubSub,
	disc p2pDisc.Discovery,
	cfg Config,
) (*share.RDANodeService, error) {
	// Parse bootstrap peer multiaddrs (if any configured).
	bootstrapPeers, err := parseRDABootstrapPeers(cfg.RDABootstrapPeers)
	if err != nil {
		return nil, err
	}

	// Only use DHT discovery if explicitly enabled in config.
	var activeDisc p2pDisc.Discovery
	if cfg.RDADiscoveryEnabled {
		activeDisc = disc
	}

	// RDA_EXPECTED_NODES env var overrides config.toml (useful in Docker).
	expectedNodes := cfg.RDAExpectedNodeCount
	if envVal := os.Getenv("RDA_EXPECTED_NODES"); envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil && n > 0 {
			expectedNodes = n
		}
	}

	// Compute grid dimensions from expected node count so they stay in sync.
	gridDims := cfg.RDAGridDimensions
	if expectedNodes > 0 {
		gridDims = share.CalculateOptimalGridSize(uint32(expectedNodes))
	}

	// Parse subnet discovery delay
	delayBeforePull := time.Duration(0)
	if cfg.RDASubnetDiscoveryDelay != "" {
		if d, err := time.ParseDuration(cfg.RDASubnetDiscoveryDelay); err == nil {
			delayBeforePull = d
		}
	}

	// Create RDA configuration
	rdaCfg := share.RDANodeServiceConfig{
		GridDimensions:        gridDims,
		ExpectedNodeCount:     uint32(expectedNodes),
		FilterPolicy:          cfg.RDAFilterPolicy,
		EnableDetailedLogging: cfg.RDADetailedLogging,
		Discovery:             activeDisc,
		BootstrapPeers:        bootstrapPeers,
		UseSubnetDiscovery:    cfg.RDAUseSubnetDiscovery,
		SubnetDiscoveryDelay:  delayBeforePull,
	}

	// Create service
	service := share.NewRDANodeService(h, ps, rdaCfg)

	// Start service
	if err := service.Start(ctx); err != nil {
		return nil, err
	}

	return service, nil
}

// parseRDABootstrapPeers converts multiaddr strings into peer.AddrInfo values.
func parseRDABootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, 0, len(addrs))
	for _, s := range addrs {
		maddr, err := ma.NewMultiaddr(s)
		if err != nil {
			return nil, err
		}
		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

// newRDAAPI creates the RDA JSON-RPC API
func newRDAAPI(service *share.RDANodeService) share.RDAAPI {
	return share.NewRDAAPI(service)
}

// newRDAModule creates the RDA module
func newRDAModule(
	service *share.RDANodeService,
	api share.RDAAPI,
) *RDAModule {
	return &RDAModule{
		Service: service,
		API:     api,
	}
}

// Compile-time check: *RDAModule implements RDANodeModule
var _ RDANodeModule = (*RDAModule)(nil)

// -- RDANodeModule interface implementation --
// All methods delegate to the underlying RDAAPI.

func (m *RDAModule) GetMyPosition(ctx context.Context) (*share.RDAPosition, error) {
	return m.API.GetMyPosition(ctx)
}

func (m *RDAModule) GetStatus(ctx context.Context) (*share.RDAStatus, error) {
	return m.API.GetStatus(ctx)
}

func (m *RDAModule) GetNodeInfo(ctx context.Context) (*share.RDANodeInfo, error) {
	return m.API.GetNodeInfo(ctx)
}

func (m *RDAModule) GetRowPeers(ctx context.Context) (*share.RDAPeerList, error) {
	return m.API.GetRowPeers(ctx)
}

func (m *RDAModule) GetColPeers(ctx context.Context) (*share.RDAPeerList, error) {
	return m.API.GetColPeers(ctx)
}

func (m *RDAModule) GetSubnetPeers(ctx context.Context) (*share.RDAPeerList, error) {
	return m.API.GetSubnetPeers(ctx)
}

func (m *RDAModule) GetGridDimensions(ctx context.Context) (*share.RDAGridInfo, error) {
	return m.API.GetGridDimensions(ctx)
}

func (m *RDAModule) GetStats(ctx context.Context) (*share.RDAStats, error) {
	return m.API.GetStats(ctx)
}

func (m *RDAModule) GetHealth(ctx context.Context) (*share.RDAHealthStatus, error) {
	return m.API.GetHealth(ctx)
}

func (m *RDAModule) PublishToSubnet(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) {
	return m.API.PublishToSubnet(ctx, req)
}

func (m *RDAModule) PublishToRow(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) {
	return m.API.PublishToRow(ctx, req)
}

func (m *RDAModule) PublishToCol(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) {
	return m.API.PublishToCol(ctx, req)
}

func (m *RDAModule) RequestDataFromRow(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error) {
	return m.API.RequestDataFromRow(ctx, req)
}

func (m *RDAModule) RequestDataFromCol(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error) {
	return m.API.RequestDataFromCol(ctx, req)
}

func (m *RDAModule) RequestDataFromSubnet(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error) {
	return m.API.RequestDataFromSubnet(ctx, req)
}

func (m *RDAModule) GetSubnetMembers(ctx context.Context) (*share.RDASubnetMembersResponse, error) {
	return m.API.GetSubnetMembers(ctx)
}

func (m *RDAModule) AnnounceToSubnet(ctx context.Context) (*share.RDAAnnouncementResponse, error) {
	return m.API.AnnounceToSubnet(ctx)
}
