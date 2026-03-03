package share

import (
	"context"
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RDANodeService aggregates all RDA components for easy integration into celestia-node
type RDANodeService struct {
	host                host.Host
	pubsub              *pubsub.PubSub
	gridManager         *RDAGridManager
	peerManager         *RDAPeerManager
	subnetManager       *RDASubnetManager
	peerFilter          *PeerFilter
	gossipRouter        *RDAGossipSubRouter
	exchangeCoordinator *RDAExchangeCoordinator

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RDANodeServiceConfig holds configuration for RDANodeService
type RDANodeServiceConfig struct {
	// Grid dimensions
	GridDimensions GridDimensions

	// Filtering policy
	FilterPolicy FilterPolicy

	// Auto-calculate grid size based on expected node count
	// If > 0, overrides GridDimensions
	ExpectedNodeCount uint32

	// Enable detailed logging
	EnableDetailedLogging bool
}

// DefaultRDANodeServiceConfig returns default configuration
func DefaultRDANodeServiceConfig() RDANodeServiceConfig {
	return RDANodeServiceConfig{
		GridDimensions:        DefaultGridDimensions,
		FilterPolicy:          DefaultFilterPolicy(),
		ExpectedNodeCount:     0,
		EnableDetailedLogging: false,
	}
}

// NewRDANodeService creates a new RDA node service
func NewRDANodeService(
	host host.Host,
	pubsub *pubsub.PubSub,
	config RDANodeServiceConfig,
) *RDANodeService {
	ctx, cancel := context.WithCancel(context.Background())

	// Calculate grid dimensions if needed
	gridDims := config.GridDimensions
	if config.ExpectedNodeCount > 0 {
		gridDims = CalculateOptimalGridSize(config.ExpectedNodeCount)
	}

	// Create grid manager
	gridManager := NewRDAGridManager(gridDims)

	// Create peer manager
	peerManager := NewRDAPeerManager(host, gridManager)

	// Create subnet manager
	subnetManager := NewRDASubnetManager(pubsub, gridManager, host.ID())

	// Create peer filter
	peerFilter := NewPeerFilter(host, gridManager, config.FilterPolicy)

	// Create gossip router
	gossipRouter := NewRDAGossipSubRouter(host, gridManager, peerManager, subnetManager, peerFilter)

	// Create exchange coordinator
	exchangeCoordinator := NewRDAExchangeCoordinator(host, gridManager, peerManager, subnetManager, peerFilter)

	service := &RDANodeService{
		host:                host,
		pubsub:              pubsub,
		gridManager:         gridManager,
		peerManager:         peerManager,
		subnetManager:       subnetManager,
		peerFilter:          peerFilter,
		gossipRouter:        gossipRouter,
		exchangeCoordinator: exchangeCoordinator,
		ctx:                 ctx,
		cancel:              cancel,
	}

	if config.EnableDetailedLogging {
		log.Infof("RDA Node Service initialized with grid %dx%d",
			gridDims.Rows, gridDims.Cols)
	}

	return service
}

// Start initializes and starts all RDA components
func (s *RDANodeService) Start(startCtx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Start peer manager
	if err := s.peerManager.Start(startCtx); err != nil {
		return fmt.Errorf("failed to start peer manager: %w", err)
	}

	// Start subnet manager
	if err := s.subnetManager.Start(startCtx); err != nil {
		return fmt.Errorf("failed to start subnet manager: %w", err)
	}

	// Start exchange coordinator
	if err := s.exchangeCoordinator.Start(); err != nil {
		return fmt.Errorf("failed to start exchange coordinator: %w", err)
	}

	log.Infof("RDA Node Service started successfully")
	return nil
}

// Stop stops all RDA components
func (s *RDANodeService) Stop(stopCtx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancel()

	// Stop exchange coordinator
	s.exchangeCoordinator.Stop()

	// Stop subnet manager
	s.subnetManager.Stop(stopCtx)

	// Stop peer manager
	s.peerManager.Stop(stopCtx)

	s.wg.Wait()

	log.Infof("RDA Node Service stopped")
	return nil
}

// GetGridManager returns the grid manager
func (s *RDANodeService) GetGridManager() *RDAGridManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gridManager
}

// GetPeerManager returns the peer manager
func (s *RDANodeService) GetPeerManager() *RDAPeerManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerManager
}

// GetSubnetManager returns the subnet manager
func (s *RDANodeService) GetSubnetManager() *RDASubnetManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subnetManager
}

// GetPeerFilter returns the peer filter
func (s *RDANodeService) GetPeerFilter() *PeerFilter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerFilter
}

// GetGossipRouter returns the gossip router
func (s *RDANodeService) GetGossipRouter() *RDAGossipSubRouter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gossipRouter
}

// GetExchangeCoordinator returns the exchange coordinator
func (s *RDANodeService) GetExchangeCoordinator() *RDAExchangeCoordinator {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exchangeCoordinator
}

// GetMyPosition returns this node's position in the grid
func (s *RDANodeService) GetMyPosition() GridPosition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerManager.GetMyPosition()
}

// GetRowPeers returns peers in the same row
func (s *RDANodeService) GetRowPeers() []peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerManager.GetRowPeers()
}

// GetColPeers returns peers in the same column
func (s *RDANodeService) GetColPeers() []peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerManager.GetColPeers()
}

// GetSubnetPeers returns all peers in the subnet (row + column)
func (s *RDANodeService) GetSubnetPeers() []peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerManager.GetSubnetPeers()
}

// PublishToSubnet publishes data to the entire subnet
func (s *RDANodeService) PublishToSubnet(ctx context.Context, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subnetManager.PublishToSubnet(ctx, data)
}

// PublishToRow publishes data to row peers
func (s *RDANodeService) PublishToRow(ctx context.Context, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subnetManager.PublishToRow(ctx, data)
}

// PublishToCol publishes data to column peers
func (s *RDANodeService) PublishToCol(ctx context.Context, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subnetManager.PublishToCol(ctx, data)
}

// RequestDataFromRow requests data from row peers
func (s *RDANodeService) RequestDataFromRow(dataHash []byte) <-chan exchangeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exchangeCoordinator.RequestFromRow(dataHash)
}

// RequestDataFromCol requests data from column peers
func (s *RDANodeService) RequestDataFromCol(dataHash []byte) <-chan exchangeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exchangeCoordinator.RequestFromCol(dataHash)
}

// GetStatus returns status information about the RDA node
func (s *RDANodeService) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rowPeers := s.peerManager.GetRowPeers()
	colPeers := s.peerManager.GetColPeers()
	position := s.peerManager.GetMyPosition()
	gridDims := s.gridManager.GetGridDimensions()

	return map[string]interface{}{
		"position":           position,
		"grid_dimensions":    gridDims,
		"row_peers":          len(rowPeers),
		"col_peers":          len(colPeers),
		"total_subnet_peers": len(s.peerManager.GetSubnetPeers()),
		"router_stats":       s.gossipRouter.GetStats(),
	}
}

// OnPeerConnected returns channel for peer connection events
func (s *RDANodeService) OnPeerConnected() <-chan peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerManager.OnPeerConnected()
}

// OnPeerDisconnected returns channel for peer disconnection events
func (s *RDANodeService) OnPeerDisconnected() <-chan peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerManager.OnPeerDisconnected()
}

// SetFilterPolicy updates the peer filtering policy
func (s *RDANodeService) SetFilterPolicy(policy FilterPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peerFilter.SetPolicy(policy)
}

// GetFilterStats returns statistics about peer filtering
func (s *RDANodeService) GetFilterStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerFilter.GetStats(s.gridManager)
}

// IsValidPeer checks if a peer is valid for communication
func (s *RDANodeService) IsValidPeer(peerID peer.ID) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerFilter.CanCommunicate(peerID)
}

// FilterPeerList filters a list of peers to only include valid ones
func (s *RDANodeService) FilterPeerList(peers []peer.ID) []peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerFilter.FilterPeers(peers)
}

// RDAIntegrationExample shows how to integrate RDA into nodebuilder
// This is just an example - actual integration would be in nodebuilder/p2p/module.go
/*
func NewRDAModule(cfg *RDANodeServiceConfig) fx.Option {
	return fx.Options(
		fx.Provide(
			func(host host.Host, ps *pubsub.PubSub) *RDANodeService {
				return NewRDANodeService(host, ps, *cfg)
			},
		),
		fx.Invoke(
			func(lc fx.Lifecycle, rda *RDANodeService) {
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						return rda.Start(ctx)
					},
					OnStop: func(ctx context.Context) error {
						return rda.Stop(ctx)
					},
				})
			},
		),
	)
}

// In nodebuilder/default_services.go, add RDANodeService to the node type:
type Node struct {
	// ... existing fields ...
	RDAService *RDANodeService
}

// In nodebuilder/node.go, add RDANodeService to provide it:
func provide(cfg *Config) fx.Option {
	return fx.Options(
		// ... existing providers ...
		fx.Provide(func() *RDANodeServiceConfig {
			return &RDANodeServiceConfig{
				ExpectedNodeCount: 10000,
				FilterPolicy: DefaultFilterPolicy(),
				EnableDetailedLogging: true,
			}
		}),
		NewRDAModule(rdaCfg),
	)
}
*/
