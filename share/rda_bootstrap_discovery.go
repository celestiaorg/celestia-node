package share

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"

	logging "github.com/ipfs/go-log/v2"
)

// Use package logger
var rdalog = logging.Logger("rda")

const (
	// RDA bootstrap discovery protocol
	RDABootstrapProtocol protocol.ID = "/celestia/rda/bootstrap/1.0.0"

	// RDA bootstrap message types
	JoinRowSubnetRequest  = "join_row"
	JoinColSubnetRequest  = "join_col"
	GetRowPeersRequest    = "get_row_peers"
	GetColPeersRequest    = "get_col_peers"
	JoinRowSubnetResponse = "join_row_resp"
	JoinColSubnetResponse = "join_col_resp"
	GetRowPeersResponse   = "get_row_peers_resp"
	GetColPeersResponse   = "get_col_peers_resp"

	// RoutingTable capacity limit per subnet (row or column)
	// Matches Ethereum KZG commitment scheme (20-50 nodes per bucket)
	RoutingTableCapacity = 50
	// Random peers returned per query (similar to DHT K parameter)
	RandomPeersPerQuery = 5
)

// BootstrapPeerRequest is sent by nodes to bootstrap nodes for peer discovery
type BootstrapPeerRequest struct {
	Type      string         // Type of request (join_row, get_row_peers, etc.)
	NodeID    string         // Requesting node's peer ID
	PeerAddrs []string       // Requesting node's multiaddrs
	Row       uint32         // Node's row in grid
	Col       uint32         // Node's column in grid
	GridDims  GridDimensions // Grid dimensions for validation
}

// BootstrapPeerResponse is sent by bootstrap nodes with peer discovery results
type BootstrapPeerResponse struct {
	Type      string          // Type of response
	Success   bool            // Whether request succeeded
	Message   string          // Status message
	RowPeers  []BootstrapPeer // Peers in same row
	ColPeers  []BootstrapPeer // Peers in same column
	Timestamp int64           // Response timestamp
}

// BootstrapPeer contains information about a discovered peer
type BootstrapPeer struct {
	PeerID    string   // Peer ID string
	Addresses []string // Multiaddrs as strings
	Position  struct {
		Row uint32
		Col uint32
	}
}

// PeerInfo stores information about a discovered peer including its grid position
type PeerInfo struct {
	AddrInfo peer.AddrInfo
	Row      uint32
	Col      uint32
}

// RoutingTableEntry represents a peer entry in the routing table
type RoutingTableEntry struct {
	PeerID   string
	AddrInfo peer.AddrInfo
	Row      uint32
	Col      uint32
	Added    time.Time // Timestamp for LRU eviction
}

// RoutingTable manages peers with capacity limits per row/column
// Implements Kademlia-like bucket behavior: max 50 peers per row/col
type RoutingTable struct {
	mu       sync.RWMutex
	rows     map[uint32][]*RoutingTableEntry // Row ID → list of peers
	cols     map[uint32][]*RoutingTableEntry // Col ID → list of peers
	capacity int                             // Max peers per row/col
}

// NewRoutingTable creates a new routing table with specified capacity
func NewRoutingTable(capacity int) *RoutingTable {
	if capacity <= 0 {
		capacity = RoutingTableCapacity
	}
	return &RoutingTable{
		rows:     make(map[uint32][]*RoutingTableEntry),
		cols:     make(map[uint32][]*RoutingTableEntry),
		capacity: capacity,
	}
}

// AddPeer adds or updates a peer in the routing table
// If the row/col bucket is full, randomly evicts an old peer
func (rt *RoutingTable) AddPeer(peerID string, addrInfo peer.AddrInfo, row, col uint32) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	entry := &RoutingTableEntry{
		PeerID:   peerID,
		AddrInfo: addrInfo,
		Row:      row,
		Col:      col,
		Added:    time.Now(),
	}

	// Add to row bucket
	if rt.rows[row] == nil {
		rt.rows[row] = make([]*RoutingTableEntry, 0, rt.capacity)
	}

	// Check if peer already exists and update
	for i, peer := range rt.rows[row] {
		if peer.PeerID == peerID {
			rt.rows[row][i] = entry
			break
		}
	}

	// If not found, add new peer
	found := false
	for _, peer := range rt.rows[row] {
		if peer.PeerID == peerID {
			found = true
			break
		}
	}

	if !found {
		if len(rt.rows[row]) < rt.capacity {
			rt.rows[row] = append(rt.rows[row], entry)
		} else {
			// Evict oldest peer (LRU)
			rt.rows[row] = rt.rows[row][1:]
			rt.rows[row] = append(rt.rows[row], entry)
		}
	}

	// Add to col bucket (same logic)
	if rt.cols[col] == nil {
		rt.cols[col] = make([]*RoutingTableEntry, 0, rt.capacity)
	}

	found = false
	for i, peer := range rt.cols[col] {
		if peer.PeerID == peerID {
			rt.cols[col][i] = entry
			found = true
			break
		}
	}

	if !found {
		if len(rt.cols[col]) < rt.capacity {
			rt.cols[col] = append(rt.cols[col], entry)
		} else {
			// Evict oldest peer (LRU)
			rt.cols[col] = rt.cols[col][1:]
			rt.cols[col] = append(rt.cols[col], entry)
		}
	}

	rdalog.Infof("RDA: routing table added peer %s at (row=%d, col=%d), row_size=%d, col_size=%d",
		peerID, row, col, len(rt.rows[row]), len(rt.cols[col]))
}

// GetRowPeers returns random peers from the specified row (excluding requester)
func (rt *RoutingTable) GetRowPeers(row uint32, count int, excludePeerID string) []*RoutingTableEntry {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	peers := rt.rows[row]
	if peers == nil {
		return []*RoutingTableEntry{}
	}

	// Filter out requester
	available := make([]*RoutingTableEntry, 0, len(peers))
	for _, p := range peers {
		if p.PeerID != excludePeerID {
			available = append(available, p)
		}
	}

	// If fewer peers than count, return all
	if len(available) <= count {
		return available
	}

	// Return random subset
	result := make([]*RoutingTableEntry, count)
	selected := make(map[int]bool)
	for i := 0; i < count; i++ {
		var idx int
		for {
			idx = randIntn(len(available))
			if !selected[idx] {
				selected[idx] = true
				break
			}
		}
		result[i] = available[idx]
	}

	return result
}

// GetColPeers returns random peers from the specified column (excluding requester)
func (rt *RoutingTable) GetColPeers(col uint32, count int, excludePeerID string) []*RoutingTableEntry {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	peers := rt.cols[col]
	if peers == nil {
		return []*RoutingTableEntry{}
	}

	// Filter out requester
	available := make([]*RoutingTableEntry, 0, len(peers))
	for _, p := range peers {
		if p.PeerID != excludePeerID {
			available = append(available, p)
		}
	}

	// If fewer peers than count, return all
	if len(available) <= count {
		return available
	}

	// Return random subset
	result := make([]*RoutingTableEntry, count)
	selected := make(map[int]bool)
	for i := 0; i < count; i++ {
		var idx int
		for {
			idx = randIntn(len(available))
			if !selected[idx] {
				selected[idx] = true
				break
			}
		}
		result[i] = available[idx]
	}

	return result
}

// Size returns the total number of unique peers in the routing table
func (rt *RoutingTable) Size() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	present := make(map[string]bool)
	for _, peers := range rt.rows {
		for _, p := range peers {
			present[p.PeerID] = true
		}
	}
	return len(present)
}

// BootstrapDiscoveryService handles peer discovery using bootstrap nodes
type BootstrapDiscoveryService struct {
	host           host.Host
	bootstrapPeers []peer.AddrInfo
	gridManager    *RDAGridManager
	myRow          uint32
	myCol          uint32

	mu           sync.RWMutex
	routingTable *RoutingTable            // Routing table with capacity limits
	rowPeers     map[string]peer.AddrInfo // Map of peer ID to peer info for row (client-side cache)
	colPeers     map[string]peer.AddrInfo // Map of peer ID to peer info for column (client-side cache)

	done chan struct{}
}

// NewBootstrapDiscoveryService creates a new bootstrap discovery service
func NewBootstrapDiscoveryService(
	h host.Host,
	bootstrapPeers []peer.AddrInfo,
	gridManager *RDAGridManager,
	myRow, myCol uint32,
) *BootstrapDiscoveryService {
	return &BootstrapDiscoveryService{
		host:           h,
		bootstrapPeers: bootstrapPeers,
		gridManager:    gridManager,
		myRow:          myRow,
		myCol:          myCol,
		routingTable:   NewRoutingTable(RoutingTableCapacity),
		rowPeers:       make(map[string]peer.AddrInfo),
		colPeers:       make(map[string]peer.AddrInfo),
		done:           make(chan struct{}),
	}
}

// Start begins the bootstrap discovery process
// Sets up server-side handler to receive requests and connects to bootstrap nodes for peer discovery
func (b *BootstrapDiscoveryService) Start(ctx context.Context) error {
	// Register handler to receive bootstrap requests from other nodes
	b.host.SetStreamHandler(RDABootstrapProtocol, b.handleBootstrapRequest)
	rdalog.Infof("RDA bootstrap: registered stream handler for %s", RDABootstrapProtocol)

	if len(b.bootstrapPeers) == 0 {
		rdalog.Warnf("RDA bootstrap discovery: no bootstrap peers configured")
		return nil
	}

	rdalog.Infof("RDA bootstrap discovery: starting with %d bootstrap peer(s)", len(b.bootstrapPeers))

	// Connect to bootstrap peers in background
	// Use background context so goroutines persist after Start() returns
	bgCtx := context.Background()
	for _, bootstrap := range b.bootstrapPeers {
		go b.contactBootstrapPeer(bgCtx, bootstrap)
	}

	return nil
}

// Stop stops the bootstrap discovery service
func (b *BootstrapDiscoveryService) Stop(ctx context.Context) error {
	select {
	case <-b.done:
	default:
		close(b.done)
	}
	return nil
}

// handleBootstrapRequest handles incoming bootstrap requests from other nodes
func (b *BootstrapDiscoveryService) handleBootstrapRequest(stream network.Stream) {
	defer stream.Close()

	// Read request
	buf := make([]byte, 4096)
	n, err := stream.Read(buf)
	if err != nil {
		rdalog.Debugf("RDA bootstrap server: failed to read request: %v", err)
		return
	}

	// Unmarshal request
	req, err := unmarshalBootstrapRequest(buf[:n])
	if err != nil {
		rdalog.Debugf("RDA bootstrap server: failed to unmarshal request: %v", err)
		return
	}

	rdalog.Infof("RDA bootstrap server: received %s request from %s at (row=%d, col=%d)", req.Type, req.NodeID, req.Row, req.Col)

	// Process request based on type
	var resp *BootstrapPeerResponse
	switch req.Type {
	case JoinRowSubnetRequest:
		resp = b.processJoinRowRequest(req)
	case JoinColSubnetRequest:
		resp = b.processJoinColRequest(req)
	case GetRowPeersRequest:
		resp = b.processGetRowPeersRequest(req)
	case GetColPeersRequest:
		resp = b.processGetColPeersRequest(req)
	default:
		resp = &BootstrapPeerResponse{
			Type:      req.Type + "_resp",
			Success:   false,
			Message:   fmt.Sprintf("unknown request type: %s", req.Type),
			Timestamp: time.Now().Unix(),
		}
	}

	// Marshal response
	data, err := marshalBootstrapResponse(resp)
	if err != nil {
		rdalog.Debugf("RDA bootstrap server: failed to marshal response: %v", err)
		return
	}

	// Write response
	if _, err := stream.Write(data); err != nil {
		rdalog.Debugf("RDA bootstrap server: failed to write response: %v", err)
		return
	}

	rdalog.Infof(
		"RDA bootstrap server: responded to %s request from %s (success=%t, row_peers=%d, col_peers=%d)",
		req.Type,
		req.NodeID,
		resp.Success,
		len(resp.RowPeers),
		len(resp.ColPeers),
	)
	// Keep stream open - let client read then close
}

// processJoinRowRequest handles a node joining its row subnet
func (b *BootstrapDiscoveryService) processJoinRowRequest(req *BootstrapPeerRequest) *BootstrapPeerResponse {
	// Parse peer info
	addrInfo, err := stringsToPeerAddrInfo(req.NodeID, req.PeerAddrs)
	if err != nil {
		rdalog.Debugf("RDA bootstrap server: failed to parse peer info: %v", err)
		return &BootstrapPeerResponse{
			Type:      JoinRowSubnetResponse,
			Success:   false,
			Message:   "failed to parse peer info",
			Timestamp: time.Now().Unix(),
		}
	}

	// Add peer to routing table (will auto-evict if capacity exceeded)
	b.routingTable.AddPeer(req.NodeID, addrInfo, req.Row, req.Col)

	// Also add to rowPeers for caching/statistics
	b.mu.Lock()
	b.rowPeers[req.NodeID] = addrInfo
	b.mu.Unlock()

	// Get random peers from row (3-5) excluding requester
	randomPeers := b.routingTable.GetRowPeers(req.Row, RandomPeersPerQuery, req.NodeID)

	// Convert to response format
	rowPeers := make([]BootstrapPeer, len(randomPeers))
	for i, entry := range randomPeers {
		rowPeers[i] = BootstrapPeer{
			PeerID:    entry.PeerID,
			Addresses: addrInfoToStrings(entry.AddrInfo),
		}
		rowPeers[i].Position.Row = entry.Row
		rowPeers[i].Position.Col = entry.Col
	}

	return &BootstrapPeerResponse{
		Type:      JoinRowSubnetResponse,
		Success:   true,
		Message:   fmt.Sprintf("joined row %d, discovered %d peers", req.Row, len(rowPeers)),
		RowPeers:  rowPeers,
		Timestamp: time.Now().Unix(),
	}
}

// processJoinColRequest handles a node joining its column subnet
func (b *BootstrapDiscoveryService) processJoinColRequest(req *BootstrapPeerRequest) *BootstrapPeerResponse {
	// Parse peer info
	addrInfo, err := stringsToPeerAddrInfo(req.NodeID, req.PeerAddrs)
	if err != nil {
		rdalog.Debugf("RDA bootstrap server: failed to parse peer info: %v", err)
		return &BootstrapPeerResponse{
			Type:      JoinColSubnetResponse,
			Success:   false,
			Message:   "failed to parse peer info",
			Timestamp: time.Now().Unix(),
		}
	}

	// Add peer to routing table (will auto-evict if capacity exceeded)
	b.routingTable.AddPeer(req.NodeID, addrInfo, req.Row, req.Col)

	// Also add to colPeers for caching/statistics
	b.mu.Lock()
	b.colPeers[req.NodeID] = addrInfo
	b.mu.Unlock()

	// Get random peers from column (3-5) excluding requester
	randomPeers := b.routingTable.GetColPeers(req.Col, RandomPeersPerQuery, req.NodeID)

	// Convert to response format
	colPeers := make([]BootstrapPeer, len(randomPeers))
	for i, entry := range randomPeers {
		colPeers[i] = BootstrapPeer{
			PeerID:    entry.PeerID,
			Addresses: addrInfoToStrings(entry.AddrInfo),
		}
		colPeers[i].Position.Row = entry.Row
		colPeers[i].Position.Col = entry.Col
	}

	return &BootstrapPeerResponse{
		Type:      JoinColSubnetResponse,
		Success:   true,
		Message:   fmt.Sprintf("joined col %d, discovered %d peers", req.Col, len(colPeers)),
		ColPeers:  colPeers,
		Timestamp: time.Now().Unix(),
	}
}

// processGetRowPeersRequest returns random peers from the same row
func (b *BootstrapDiscoveryService) processGetRowPeersRequest(req *BootstrapPeerRequest) *BootstrapPeerResponse {
	// Get random peers from row (3-5) excluding requester
	randomPeers := b.routingTable.GetRowPeers(req.Row, RandomPeersPerQuery, req.NodeID)

	// Convert to response format
	rowPeers := make([]BootstrapPeer, len(randomPeers))
	for i, entry := range randomPeers {
		rowPeers[i] = BootstrapPeer{
			PeerID:    entry.PeerID,
			Addresses: addrInfoToStrings(entry.AddrInfo),
		}
		rowPeers[i].Position.Row = entry.Row
		rowPeers[i].Position.Col = entry.Col
	}

	return &BootstrapPeerResponse{
		Type:      GetRowPeersResponse,
		Success:   true,
		Message:   fmt.Sprintf("found %d peers in row %d", len(rowPeers), req.Row),
		RowPeers:  rowPeers,
		Timestamp: time.Now().Unix(),
	}
}

// processGetColPeersRequest returns random peers from the same column
func (b *BootstrapDiscoveryService) processGetColPeersRequest(req *BootstrapPeerRequest) *BootstrapPeerResponse {
	// Get random peers from column (3-5) excluding requester
	randomPeers := b.routingTable.GetColPeers(req.Col, RandomPeersPerQuery, req.NodeID)

	// Convert to response format
	colPeers := make([]BootstrapPeer, len(randomPeers))
	for i, entry := range randomPeers {
		colPeers[i] = BootstrapPeer{
			PeerID:    entry.PeerID,
			Addresses: addrInfoToStrings(entry.AddrInfo),
		}
		colPeers[i].Position.Row = entry.Row
		colPeers[i].Position.Col = entry.Col
	}

	return &BootstrapPeerResponse{
		Type:      GetColPeersResponse,
		Success:   true,
		Message:   fmt.Sprintf("found %d peers in col %d", len(colPeers), req.Col),
		ColPeers:  colPeers,
		Timestamp: time.Now().Unix(),
	}
}

// contactBootstrapPeer contacts a single bootstrap peer for subnet discovery
func (b *BootstrapDiscoveryService) contactBootstrapPeer(ctx context.Context, bootstrap peer.AddrInfo) {
	// Step 1: Connect to bootstrap peer
	if err := b.host.Connect(ctx, bootstrap); err != nil {
		rdalog.Debugf("RDA bootstrap: failed to connect to bootstrap peer %s: %v", bootstrap.ID, err)
		return
	}

	rdalog.Infof("RDA bootstrap: connected to bootstrap peer %s", bootstrap.ID)

	// Step 2: Send JOIN request to row subnet
	rdalog.Infof("RDA bootstrap: sending %s request (row=%d, col=%d) to %s", JoinRowSubnetRequest, b.myRow, b.myCol, bootstrap.ID)
	if err := b.sendJoinRequest(ctx, bootstrap.ID, JoinRowSubnetRequest, b.myRow, b.myCol); err != nil {
		rdalog.Debugf("RDA bootstrap: failed to join row subnet: %v", err)
	}

	// Step 3: Send JOIN request to column subnet
	rdalog.Infof("RDA bootstrap: sending %s request (row=%d, col=%d) to %s", JoinColSubnetRequest, b.myRow, b.myCol, bootstrap.ID)
	if err := b.sendJoinRequest(ctx, bootstrap.ID, JoinColSubnetRequest, b.myRow, b.myCol); err != nil {
		rdalog.Debugf("RDA bootstrap: failed to join col subnet: %v", err)
	}

	// Step 4: Request row peers
	rdalog.Infof("RDA bootstrap: requesting row peers from %s for row=%d", bootstrap.ID, b.myRow)
	if rowPeers, err := b.requestPeersFromBootstrap(ctx, bootstrap.ID, GetRowPeersRequest, b.myRow, b.myCol); err == nil {
		b.mu.Lock()
		for _, p := range rowPeers {
			if addrInfo, err := peerToAddrInfo(p); err == nil {
				b.rowPeers[p.PeerID] = addrInfo
			}
		}
		b.mu.Unlock()
		rdalog.Infof("RDA bootstrap: discovered %d row peers from bootstrap %s", len(rowPeers), bootstrap.ID)
	} else {
		rdalog.Debugf("RDA bootstrap: failed to get row peers: %v", err)
	}

	// Step 5: Request column peers
	rdalog.Infof("RDA bootstrap: requesting col peers from %s for col=%d", bootstrap.ID, b.myCol)
	if colPeers, err := b.requestPeersFromBootstrap(ctx, bootstrap.ID, GetColPeersRequest, b.myRow, b.myCol); err == nil {
		b.mu.Lock()
		for _, p := range colPeers {
			if addrInfo, err := peerToAddrInfo(p); err == nil {
				b.colPeers[p.PeerID] = addrInfo
			}
		}
		b.mu.Unlock()
		rdalog.Infof("RDA bootstrap: discovered %d col peers from bootstrap %s", len(colPeers), bootstrap.ID)
	} else {
		rdalog.Debugf("RDA bootstrap: failed to get col peers: %v", err)
	}
}

// sendJoinRequest sends a join request to a bootstrap node
func (b *BootstrapDiscoveryService) sendJoinRequest(ctx context.Context, bootstrapID peer.ID, requestType string, row, col uint32) error {
	// Create stream to bootstrap node
	stream, err := b.host.NewStream(ctx, bootstrapID, RDABootstrapProtocol)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	// Prepare request
	req := BootstrapPeerRequest{
		Type:     requestType,
		NodeID:   b.host.ID().String(),
		Row:      row,
		Col:      col,
		GridDims: b.gridManager.GetGridDimensions(),
		PeerAddrs: addrInfoToStrings(peer.AddrInfo{
			ID:    b.host.ID(),
			Addrs: b.host.Addrs(),
		}),
	}

	// Marshal and send request
	data, err := marshalBootstrapRequest(&req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}
	rdalog.Infof("RDA bootstrap: sent %s request to %s (row=%d, col=%d)", requestType, bootstrapID, row, col)

	// Read response (optional for join requests)
	buf := make([]byte, 4096)
	if _, err := stream.Read(buf); err != nil && err != network.ErrReset {
		rdalog.Debugf("RDA bootstrap: no response to join request: %v", err)
	}

	return nil
}

// requestPeersFromBootstrap requests peer list from a bootstrap node
func (b *BootstrapDiscoveryService) requestPeersFromBootstrap(
	ctx context.Context,
	bootstrapID peer.ID,
	requestType string,
	row, col uint32,
) ([]BootstrapPeer, error) {
	// Create stream to bootstrap node
	stream, err := b.host.NewStream(ctx, bootstrapID, RDABootstrapProtocol)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	// Prepare request
	req := BootstrapPeerRequest{
		Type:     requestType,
		NodeID:   b.host.ID().String(),
		Row:      row,
		Col:      col,
		GridDims: b.gridManager.GetGridDimensions(),
		PeerAddrs: addrInfoToStrings(peer.AddrInfo{
			ID:    b.host.ID(),
			Addrs: b.host.Addrs(),
		}),
	}

	// Marshal and send request
	data, err := marshalBootstrapRequest(&req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := stream.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	buf := make([]byte, 65536)
	n, err := stream.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Unmarshal response
	resp, err := unmarshalBootstrapResponse(buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("bootstrap responded with error: %s", resp.Message)
	}

	// Return appropriate peer list based on request type
	switch requestType {
	case GetRowPeersRequest:
		return resp.RowPeers, nil
	case GetColPeersRequest:
		return resp.ColPeers, nil
	default:
		return nil, fmt.Errorf("unknown request type: %s", requestType)
	}
}

// GetRowPeers returns currently discovered row peers
func (b *BootstrapDiscoveryService) GetRowPeers() []peer.AddrInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()

	peers := make([]peer.AddrInfo, 0, len(b.rowPeers))
	for _, p := range b.rowPeers {
		peers = append(peers, p)
	}
	return peers
}

// GetColPeers returns currently discovered column peers
func (b *BootstrapDiscoveryService) GetColPeers() []peer.AddrInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()

	peers := make([]peer.AddrInfo, 0, len(b.colPeers))
	for _, p := range b.colPeers {
		peers = append(peers, p)
	}
	return peers
}

// Helper functions

func addrInfoToStrings(info peer.AddrInfo) []string {
	addrs := make([]string, 0, len(info.Addrs))
	for _, addr := range info.Addrs {
		addrs = append(addrs, addr.String())
	}
	// Add peer ID to last address (multiaddr format /p2p/...)
	if len(addrs) > 0 {
		addrs[len(addrs)-1] = ma.Join(
			ma.StringCast(addrs[len(addrs)-1]),
			ma.StringCast(fmt.Sprintf("/p2p/%s", info.ID)),
		).String()
	}
	return addrs
}

func peerToAddrInfo(bp BootstrapPeer) (peer.AddrInfo, error) {
	id, err := peer.Decode(bp.PeerID)
	if err != nil {
		return peer.AddrInfo{}, err
	}

	addrs := make([]ma.Multiaddr, 0, len(bp.Addresses))
	for _, addrStr := range bp.Addresses {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		addrs = append(addrs, addr)
	}

	return peer.AddrInfo{
		ID:    id,
		Addrs: addrs,
	}, nil
}

// stringsToPeerAddrInfo converts a peer ID string and address strings to peer.AddrInfo
func stringsToPeerAddrInfo(peerIDStr string, addrs []string) (peer.AddrInfo, error) {
	id, err := peer.Decode(peerIDStr)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("invalid peer ID: %w", err)
	}

	multiaddrs := make([]ma.Multiaddr, 0, len(addrs))
	for _, addrStr := range addrs {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		multiaddrs = append(multiaddrs, addr)
	}

	return peer.AddrInfo{
		ID:    id,
		Addrs: multiaddrs,
	}, nil
}

// Serialization helpers - use simple pipe-delimited format for bootstrap protocol
func marshalBootstrapRequest(req *BootstrapPeerRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Format: REQ|type|nodeID|row|col|gridrows|gridcols|addr1,addr2,...
	addrStr := ""
	if len(req.PeerAddrs) > 0 {
		for i, addr := range req.PeerAddrs {
			if i > 0 {
				addrStr += ","
			}
			addrStr += addr
		}
	}

	line := fmt.Sprintf("REQ|%s|%s|%d|%d|%d|%d|%s\n",
		req.Type, req.NodeID, req.Row, req.Col,
		req.GridDims.Rows, req.GridDims.Cols, addrStr)

	return []byte(line), nil
}

func unmarshalBootstrapRequest(data []byte) (*BootstrapPeerRequest, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty request data")
	}

	// Parse: REQ|type|nodeID|row|col|gridrows|gridcols|addr1,addr2,...
	line := string(data)
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}

	parts := split(line, '|')
	if len(parts) < 8 {
		return nil, fmt.Errorf("invalid request format: expected 8+ fields, got %d", len(parts))
	}

	if parts[0] != "REQ" {
		return nil, fmt.Errorf("invalid request prefix: %s", parts[0])
	}

	req := &BootstrapPeerRequest{
		Type:   parts[1],
		NodeID: parts[2],
	}

	// Parse row, col
	var row, col, gridRows, gridCols uint32
	if _, err := fmt.Sscanf(parts[3], "%d", &row); err != nil {
		return nil, fmt.Errorf("invalid row: %w", err)
	}
	if _, err := fmt.Sscanf(parts[4], "%d", &col); err != nil {
		return nil, fmt.Errorf("invalid col: %w", err)
	}
	if _, err := fmt.Sscanf(parts[5], "%d", &gridRows); err != nil {
		return nil, fmt.Errorf("invalid grid rows: %w", err)
	}
	if _, err := fmt.Sscanf(parts[6], "%d", &gridCols); err != nil {
		return nil, fmt.Errorf("invalid grid cols: %w", err)
	}

	req.Row = row
	req.Col = col
	req.GridDims = GridDimensions{Rows: uint16(gridRows), Cols: uint16(gridCols)}

	// Parse addresses
	if len(parts[7]) > 0 {
		addrs := split(parts[7], ',')
		req.PeerAddrs = addrs
	}

	return req, nil
}

func marshalBootstrapResponse(resp *BootstrapPeerResponse) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("response cannot be nil")
	}

	// Format: RESP|type|success|message|timestamp|rowpeercount|colpeercount|rowpeers...|colpeers...
	var output string
	successStr := "0"
	if resp.Success {
		successStr = "1"
	}

	output = fmt.Sprintf("RESP|%s|%s|%s|%d|%d|%d",
		resp.Type, successStr, resp.Message, resp.Timestamp, len(resp.RowPeers), len(resp.ColPeers))

	// Append row peers
	for _, peer := range resp.RowPeers {
		addrStr := ""
		if len(peer.Addresses) > 0 {
			for i, addr := range peer.Addresses {
				if i > 0 {
					addrStr += ","
				}
				addrStr += addr
			}
		}
		output += fmt.Sprintf("|ROWPEER|%s|%d|%d|%s",
			peer.PeerID, peer.Position.Row, peer.Position.Col, addrStr)
	}

	// Append column peers
	for _, peer := range resp.ColPeers {
		addrStr := ""
		if len(peer.Addresses) > 0 {
			for i, addr := range peer.Addresses {
				if i > 0 {
					addrStr += ","
				}
				addrStr += addr
			}
		}
		output += fmt.Sprintf("|COLPEER|%s|%d|%d|%s",
			peer.PeerID, peer.Position.Row, peer.Position.Col, addrStr)
	}

	output += "\n"
	return []byte(output), nil
}

func unmarshalBootstrapResponse(data []byte) (*BootstrapPeerResponse, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty response data")
	}

	line := string(data)
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}

	parts := split(line, '|')
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid response format: expected 6+ fields, got %d", len(parts))
	}

	if parts[0] != "RESP" {
		return nil, fmt.Errorf("invalid response prefix: %s", parts[0])
	}

	resp := &BootstrapPeerResponse{
		Type:    parts[1],
		Message: parts[3],
	}

	// Parse success flag
	resp.Success = parts[2] == "1"

	// Parse timestamp
	if _, err := fmt.Sscanf(parts[4], "%d", &resp.Timestamp); err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	// Parse row and column peer counts
	var rowCount, colCount int
	if _, err := fmt.Sscanf(parts[5], "%d", &rowCount); err != nil {
		return nil, fmt.Errorf("invalid row peer count: %w", err)
	}
	if len(parts) > 6 {
		if _, err := fmt.Sscanf(parts[6], "%d", &colCount); err != nil {
			return nil, fmt.Errorf("invalid col peer count: %w", err)
		}
	}

	// Create GridDimensions with proper uint16 conversion
	resp.RowPeers = make([]BootstrapPeer, 0, rowCount)
	resp.ColPeers = make([]BootstrapPeer, 0, colCount)

	idx := 7
	rowsProcessed := 0
	colsProcessed := 0

	for idx < len(parts) && (rowsProcessed < rowCount || colsProcessed < colCount) {
		if parts[idx] == "ROWPEER" && rowsProcessed < rowCount && idx+3 < len(parts) {
			peer := BootstrapPeer{
				PeerID: parts[idx+1],
			}
			if _, err := fmt.Sscanf(parts[idx+2], "%d", &peer.Position.Row); err != nil {
				continue
			}
			if _, err := fmt.Sscanf(parts[idx+3], "%d", &peer.Position.Col); err != nil {
				continue
			}
			if idx+4 < len(parts) && parts[idx+4] != "ROWPEER" && parts[idx+4] != "COLPEER" {
				addrs := split(parts[idx+4], ',')
				peer.Addresses = addrs
				idx += 5
			} else {
				idx += 4
			}
			resp.RowPeers = append(resp.RowPeers, peer)
			rowsProcessed++
		} else if parts[idx] == "COLPEER" && colsProcessed < colCount && idx+3 < len(parts) {
			peer := BootstrapPeer{
				PeerID: parts[idx+1],
			}
			if _, err := fmt.Sscanf(parts[idx+2], "%d", &peer.Position.Row); err != nil {
				continue
			}
			if _, err := fmt.Sscanf(parts[idx+3], "%d", &peer.Position.Col); err != nil {
				continue
			}
			if idx+4 < len(parts) && parts[idx+4] != "ROWPEER" && parts[idx+4] != "COLPEER" {
				addrs := split(parts[idx+4], ',')
				peer.Addresses = addrs
				idx += 5
			} else {
				idx += 4
			}
			resp.ColPeers = append(resp.ColPeers, peer)
			colsProcessed++
		} else {
			idx++
		}
	}

	return resp, nil
}

// Helper function to generate a random integer in [0, n)
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	// Use crypto/rand for secure random number generation
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	// Convert 4 random bytes to uint32 and mod by n
	val := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return int(val % uint32(n))
}

// Helper function to split strings manually (avoiding depend on extra imports)
func split(s string, sep byte) []string {
	parts := make([]string, 0)
	current := ""

	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(s[i])
		}
	}
	parts = append(parts, current)
	return parts
}
