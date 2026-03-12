package share

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2pDisc "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RDANodeService aggregates all RDA components for easy integration into celestia-node
type RDANodeService struct {
	host                   host.Host
	pubsub                 *pubsub.PubSub
	gridManager            *RDAGridManager
	peerManager            *RDAPeerManager
	subnetManager          *RDASubnetManager
	peerFilter             *PeerFilter
	gossipRouter           *RDAGossipSubRouter
	exchangeCoordinator    *RDAExchangeCoordinator
	discovery              *RDADiscovery              // nil if discovery disabled
	bootstrapDiscovery     *BootstrapDiscoveryService // Bootstrap-based peer discovery
	subnetDiscoveryManager *RDASubnetDiscoveryManager
	useSubnetDiscovery     bool // Use paper-based subnet protocol instead of DHT
	useBootstrapDiscovery  bool // Use bootstrap nodes for peer discovery

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

	// Discovery is a libp2p discovery.Discovery (backed by DHT) used to
	// find row/col peers via rendezvous. If nil, DHT-based discovery is
	// disabled (useful for tests using mocknet).
	Discovery libp2pDisc.Discovery

	// BootstrapPeers is the list of known stable peers (typically bridge nodes)
	// to connect to immediately on Start, so DHT discovery can follow.
	BootstrapPeers []peer.AddrInfo

	// UseSubnetDiscovery enables RDA paper-based subnet discovery protocol
	// instead of DHT. When true, nodes join via bootstrap and discover peers
	// through subnet announcements/gossip.
	UseSubnetDiscovery bool

	// SubnetDiscoveryDelay is the delay before pulling full membership list
	// after joining a subnet (implements "delayed pull" from paper).
	SubnetDiscoveryDelay time.Duration
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

// Validate checks if the configuration is valid
func (c *RDANodeServiceConfig) Validate() error {
	if c.GridDimensions.Rows == 0 || c.GridDimensions.Cols == 0 {
		return fmt.Errorf("grid dimensions must be positive: rows=%d, cols=%d", c.GridDimensions.Rows, c.GridDimensions.Cols)
	}

	if c.GridDimensions.Rows > 10000 || c.GridDimensions.Cols > 10000 {
		return fmt.Errorf("grid dimensions too large: rows=%d, cols=%d (max 10000)", c.GridDimensions.Rows, c.GridDimensions.Cols)
	}

	if c.SubnetDiscoveryDelay < 0 {
		return fmt.Errorf("subnet discovery delay cannot be negative: %v", c.SubnetDiscoveryDelay)
	}

	return nil
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

	// Optionally create DHT-backed discovery service.
	var disc *RDADiscovery
	if config.Discovery != nil && !config.UseSubnetDiscovery {
		disc = NewRDADiscovery(host, config.Discovery, gridManager, config.BootstrapPeers)
	}

	// Create bootstrap discovery service (always listens for incoming requests)
	// Will only contact bootstrap peers if config.BootstrapPeers is non-empty
	myCoords := GetCoords(host.ID(), gridDims)
	bootstrapDisc := NewBootstrapDiscoveryService(host, config.BootstrapPeers, gridManager, uint32(myCoords.Row), uint32(myCoords.Col))
	useBootstrapDiscovery := true

	// Create subnet discovery manager if enabled
	delayBeforePull := config.SubnetDiscoveryDelay
	if delayBeforePull == 0 {
		delayBeforePull = 4 * time.Second // Default: ~4 rounds
	}
	subnetDiscoveryMgr := NewRDASubnetDiscoveryManager(host, pubsub, delayBeforePull)

	service := &RDANodeService{
		host:                   host,
		pubsub:                 pubsub,
		gridManager:            gridManager,
		peerManager:            peerManager,
		subnetManager:          subnetManager,
		peerFilter:             peerFilter,
		bootstrapDiscovery:     bootstrapDisc,
		useBootstrapDiscovery:  useBootstrapDiscovery,
		gossipRouter:           gossipRouter,
		exchangeCoordinator:    exchangeCoordinator,
		discovery:              disc,
		subnetDiscoveryManager: subnetDiscoveryMgr,
		useSubnetDiscovery:     config.UseSubnetDiscovery,
		ctx:                    ctx,
		cancel:                 cancel,
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

	// Start DHT discovery (if configured and not using subnet discovery)
	if s.discovery != nil && !s.useSubnetDiscovery {
		if err := s.discovery.Start(startCtx); err != nil {
			return fmt.Errorf("failed to start RDA discovery: %w", err)
		}
	}

	// If using subnet discovery, announce to subnets
	if s.useSubnetDiscovery {
		go s.startSubnetDiscovery(startCtx)
	}

	log.Infof("RDA Node Service started successfully")
	return nil
}

// Stop stops all RDA components
func (s *RDANodeService) Stop(stopCtx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancel()

	// Stop discovery first so no new connections are attempted
	if s.discovery != nil {
		if err := s.discovery.Stop(stopCtx); err != nil {
			log.Warnf("RDA discovery stop error: %v", err)
		}
	}

	// Stop bootstrap discovery service
	if s.bootstrapDiscovery != nil {
		if err := s.bootstrapDiscovery.Stop(stopCtx); err != nil {
			log.Warnf("RDA bootstrap discovery stop error: %v", err)
		}
	}

	// Stop subnet discovery manager
	if err := s.subnetDiscoveryManager.Stop(stopCtx); err != nil {
		log.Warnf("RDA subnet discovery stop error: %v", err)
	}

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

// GetSubnetDiscoveryManager returns the subnet discovery manager
func (s *RDANodeService) GetSubnetDiscoveryManager() *RDASubnetDiscoveryManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subnetDiscoveryManager
}

// startSubnetDiscovery initiates the subnet discovery protocol with bootstrap nodes
// Step 1: Contact bootstrap nodes to join row/column subnets
// Step 2: Get peer lists from bootstrap nodes
// Step 3: Announce to subnets via gossip (for redundancy)
// Step 4: Connect to all discovered peers
func (s *RDANodeService) startSubnetDiscovery(startCtx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	// Get this node's grid position
	myPos := GetCoords(s.host.ID(), s.gridManager.GetGridDimensions())
	rowSubnet := fmt.Sprintf("row/%d", myPos.Row)
	colSubnet := fmt.Sprintf("col/%d", myPos.Col)

	log.Infof("RDA Subnet Discovery: node at (row=%d, col=%d)", myPos.Row, myPos.Col)

	// ========== STEP 1 & 2: Bootstrap-based discovery ==========
	// All nodes always listen for bootstrap requests (server mode)
	// Only nodes with configured bootstrap peers will contact them (client mode)
	var bootstrapRowPeers []peer.AddrInfo
	var bootstrapColPeers []peer.AddrInfo

	if err := s.bootstrapDiscovery.Start(startCtx); err != nil {
		log.Warnf("RDA bootstrap discovery failed to start: %v", err)
	} else {
		// Wait for bootstrap discovery to complete (only contacts if bootstrap peers configured)
		time.Sleep(3 * time.Second)
		bootstrapRowPeers = s.bootstrapDiscovery.GetRowPeers()
		bootstrapColPeers = s.bootstrapDiscovery.GetColPeers()
		log.Infof("RDA bootstrap discovered: %d row peers, %d col peers", len(bootstrapRowPeers), len(bootstrapColPeers))
	}

	// ========== STEP 3: Gossip-based discovery (for redundancy) ==========
	var gossipRowPeers []SubnetMember
	var gossipColPeers []SubnetMember

	// Create announcers for row and column subnets
	rowAnnouncer, err := s.subnetDiscoveryManager.GetOrCreateAnnouncer(startCtx, rowSubnet)
	if err != nil {
		log.Warnf("RDA subnet discovery: failed to create row announcer: %v", err)
	} else {
		// Announce presence in row subnet
		if err := rowAnnouncer.AnnounceJoin(startCtx); err != nil {
			log.Warnf("RDA subnet discovery: failed to announce to row subnet: %v", err)
		}
		// Collect gossip-based members
		gossipRowPeers = rowAnnouncer.GetMembersAfterDelay(startCtx)
	}

	colAnnouncer, err := s.subnetDiscoveryManager.GetOrCreateAnnouncer(startCtx, colSubnet)
	if err != nil {
		log.Warnf("RDA subnet discovery: failed to create col announcer: %v", err)
	} else {
		// Announce presence in column subnet
		if err := colAnnouncer.AnnounceJoin(startCtx); err != nil {
			log.Warnf("RDA subnet discovery: failed to announce to col subnet: %v", err)
		}
		// Collect gossip-based members
		gossipColPeers = colAnnouncer.GetMembersAfterDelay(startCtx)
	}

	log.Infof("RDA Subnet Discovery: gossip found %d row peers, %d col peers",
		len(gossipRowPeers), len(gossipColPeers))

	// ========== STEP 4: Combine bootstrap + gossip results & connect ==========
	s.connectToAllDiscoveredPeers(startCtx, bootstrapRowPeers, bootstrapColPeers, gossipRowPeers, gossipColPeers)

	// Monitor for new members via gossip
	if rowAnnouncer != nil && colAnnouncer != nil {
		go s.monitorSubnetMembership(rowAnnouncer, colAnnouncer)
	}
}

// connectToAllDiscoveredPeers combines bootstrap and gossip discovery results and connects
func (s *RDANodeService) connectToAllDiscoveredPeers(
	ctx context.Context,
	bootstrapRowPeers, bootstrapColPeers []peer.AddrInfo,
	gossipRowPeers, gossipColPeers []SubnetMember,
) {
	// Merge bootstrap peers (already in AddrInfo format)
	rowPeersMap := make(map[peer.ID]peer.AddrInfo)
	colPeersMap := make(map[peer.ID]peer.AddrInfo)

	// Add bootstrap peers
	for _, p := range bootstrapRowPeers {
		rowPeersMap[p.ID] = p
	}
	for _, p := range bootstrapColPeers {
		colPeersMap[p.ID] = p
	}

	// Add gossip peers (SubnetMember already has peer.AddrInfo)
	for _, m := range gossipRowPeers {
		for _, addrInfo := range m.PeerAddrs {
			// Merge addresses if peer already exists
			if existing, ok := rowPeersMap[m.PeerID]; ok {
				existing.Addrs = append(existing.Addrs, addrInfo.Addrs...)
				rowPeersMap[m.PeerID] = existing
			} else {
				rowPeersMap[m.PeerID] = addrInfo
			}
		}
	}
	for _, m := range gossipColPeers {
		for _, addrInfo := range m.PeerAddrs {
			// Merge addresses if peer already exists
			if existing, ok := colPeersMap[m.PeerID]; ok {
				existing.Addrs = append(existing.Addrs, addrInfo.Addrs...)
				colPeersMap[m.PeerID] = existing
			} else {
				colPeersMap[m.PeerID] = addrInfo
			}
		}
	}

	// Convert maps to slices
	var rowPeers, colPeers []peer.AddrInfo
	for _, p := range rowPeersMap {
		rowPeers = append(rowPeers, p)
	}
	for _, p := range colPeersMap {
		colPeers = append(colPeers, p)
	}

	log.Infof(
		"RDA: discovery sources summary - bootstrap(row=%d,col=%d), gossip(row=%d,col=%d)",
		len(bootstrapRowPeers),
		len(bootstrapColPeers),
		len(gossipRowPeers),
		len(gossipColPeers),
	)
	log.Infof("RDA: total discovered peers - rows: %d, cols: %d", len(rowPeers), len(colPeers))

	// Connect to discovered members
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Connect to row members
	for _, p := range rowPeers {
		go func(peerInfo peer.AddrInfo) {
			log.Infof("RDA: attempting row peer connect to %s", peerInfo.ID.String())
			if err := s.host.Connect(connCtx, peerInfo); err != nil {
				log.Infof("RDA: failed to connect row peer %s: %v", peerInfo.ID.String(), err)
			} else {
				log.Infof("RDA: connected to row peer %s", peerInfo.ID.String())
			}
		}(p)
	}

	// Connect to column members
	for _, p := range colPeers {
		go func(peerInfo peer.AddrInfo) {
			log.Infof("RDA: attempting col peer connect to %s", peerInfo.ID.String())
			if err := s.host.Connect(connCtx, peerInfo); err != nil {
				log.Infof("RDA: failed to connect col peer %s: %v", peerInfo.ID.String(), err)
			} else {
				log.Infof("RDA: connected to col peer %s", peerInfo.ID.String())
			}
		}(p)
	}
}

// connectToSubnetMembers establishes connections to discovered subnet members
func (s *RDANodeService) connectToSubnetMembers(ctx context.Context,
	rowMembers, colMembers []SubnetMember) {

	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Connect to row members
	for _, member := range rowMembers {
		go func(m SubnetMember) {
			for _, addrInfo := range m.PeerAddrs {
				if err := s.host.Connect(connCtx, addrInfo); err != nil {
					log.Debugf("RDA: failed to connect row member %s: %v", m.PeerID, err)
					continue
				}
				log.Debugf("RDA: connected to row member %s", m.PeerID)
				break
			}
		}(member)
	}

	// Connect to column members
	for _, member := range colMembers {
		go func(m SubnetMember) {
			for _, addrInfo := range m.PeerAddrs {
				if err := s.host.Connect(connCtx, addrInfo); err != nil {
					log.Debugf("RDA: failed to connect col member %s: %v", m.PeerID, err)
					continue
				}
				log.Debugf("RDA: connected to col member %s", m.PeerID)
				break
			}
		}(member)
	}
}

// monitorSubnetMembership continuously monitors for new members joining subnets
func (s *RDANodeService) monitorSubnetMembership(
	rowAnnouncer, colAnnouncer *SubnetAnnouncer) {

	for {
		select {
		case <-s.ctx.Done():
			return
		case member := <-rowAnnouncer.MemberUpdates():
			log.Debugf("RDA: new row member detected: %s", member.PeerID)
			go func(m SubnetMember) {
				for _, addr := range m.PeerAddrs {
					_ = s.host.Connect(context.Background(), addr)
				}
			}(member)
		case member := <-colAnnouncer.MemberUpdates():
			log.Debugf("RDA: new col member detected: %s", member.PeerID)
			go func(m SubnetMember) {
				for _, addr := range m.PeerAddrs {
					_ = s.host.Connect(context.Background(), addr)
				}
			}(member)
		}
	}
}

// GetDiscovery returns the RDA discovery service (nil if disabled).
func (s *RDANodeService) GetDiscovery() *RDADiscovery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.discovery
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
