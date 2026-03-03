package share

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("rda")

// RDAPeerManager quản lý peer discovery dựa trên grid RDA
type RDAPeerManager struct {
	host        host.Host
	gridManager *RDAGridManager
	myPeerID    peer.ID
	myPosition  GridPosition

	// Tracking peers
	rowPeers map[int]map[peer.ID]peerInfo
	colPeers map[int]map[peer.ID]peerInfo
	mu       sync.RWMutex

	// Events
	peerConnectedCh    chan peer.ID
	peerDisconnectedCh chan peer.ID

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type peerInfo struct {
	id           peer.ID
	position     GridPosition
	lastSeen     time.Time
	addressCount int
}

// NewRDAPeerManager creates a new RDA peer manager
func NewRDAPeerManager(
	host host.Host,
	gridManager *RDAGridManager,
) *RDAPeerManager {
	ctx, cancel := context.WithCancel(context.Background())

	pm := &RDAPeerManager{
		host:               host,
		gridManager:        gridManager,
		myPeerID:           host.ID(),
		rowPeers:           make(map[int]map[peer.ID]peerInfo),
		colPeers:           make(map[int]map[peer.ID]peerInfo),
		peerConnectedCh:    make(chan peer.ID, 100),
		peerDisconnectedCh: make(chan peer.ID, 100),
		ctx:                ctx,
		cancel:             cancel,
	}

	pm.myPosition = gridManager.RegisterPeer(pm.myPeerID)
	pm.initializePeerMaps()

	return pm
}

// initializePeerMaps initialize the peer maps for all grid positions
func (pm *RDAPeerManager) initializePeerMaps() {
	dims := pm.gridManager.GetGridDimensions()

	// Initialize row peer maps
	for r := 0; r < int(dims.Rows); r++ {
		pm.rowPeers[r] = make(map[peer.ID]peerInfo)
	}

	// Initialize column peer maps
	for c := 0; c < int(dims.Cols); c++ {
		pm.colPeers[c] = make(map[peer.ID]peerInfo)
	}
}

// Start begins listening for peer events
func (pm *RDAPeerManager) Start(ctx context.Context) error {
	pm.wg.Add(1)
	go pm.listenPeerEvents()
	log.Infof("RDA peer manager started for peer %s at position (%d, %d)",
		pm.myPeerID.String()[:16], pm.myPosition.Row, pm.myPosition.Col)
	return nil
}

// Stop stops the peer manager
func (pm *RDAPeerManager) Stop(ctx context.Context) error {
	pm.cancel()
	pm.wg.Wait()
	close(pm.peerConnectedCh)
	close(pm.peerDisconnectedCh)
	log.Infof("RDA peer manager stopped")
	return nil
}

// listenPeerEvents listens for peer connection/disconnection events
func (pm *RDAPeerManager) listenPeerEvents() {
	defer pm.wg.Done()

	subChan, err := pm.host.EventBus().Subscribe([]interface{}{
		new(event.EvtPeerConnectednessChanged),
	})
	if err != nil {
		log.Errorf("failed to subscribe to peer events: %v", err)
		return
	}
	defer subChan.Close()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case evt := <-subChan.Out():
			if e, ok := evt.(event.EvtPeerConnectednessChanged); ok {
				pm.handlePeerConnectednessChange(e.Peer, e.Connectedness)
			}
		}
	}
}

// handlePeerConnectednessChange handles peer connectivity changes
func (pm *RDAPeerManager) handlePeerConnectednessChange(peerID peer.ID, connectedness network.Connectedness) {
	// Only track peers in same row or column
	if !IsInSameSubnet(pm.myPeerID, peerID, pm.gridManager.GetGridDimensions()) {
		return
	}

	position := pm.gridManager.RegisterPeer(peerID)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	info := peerInfo{
		id:       peerID,
		position: position,
		lastSeen: time.Now(),
	}

	if connectedness == network.Connected {
		// Add peer to row and column maps
		pm.rowPeers[position.Row][peerID] = info
		pm.colPeers[position.Col][peerID] = info

		log.Debugf("peer %s connected at position (%d, %d)",
			peerID.String()[:16], position.Row, position.Col)

		select {
		case pm.peerConnectedCh <- peerID:
		default:
		}
	} else if connectedness == network.NotConnected {
		// Remove peer from row and column maps
		delete(pm.rowPeers[position.Row], peerID)
		delete(pm.colPeers[position.Col], peerID)

		log.Debugf("peer %s disconnected from position (%d, %d)",
			peerID.String()[:16], position.Row, position.Col)

		select {
		case pm.peerDisconnectedCh <- peerID:
		default:
		}
	}
}

// GetRowPeers returns all connected peers in the same row
func (pm *RDAPeerManager) GetRowPeers() []peer.ID {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var peers []peer.ID
	if rowMap, ok := pm.rowPeers[pm.myPosition.Row]; ok {
		for peerID := range rowMap {
			peers = append(peers, peerID)
		}
	}
	return peers
}

// GetColPeers returns all connected peers in the same column
func (pm *RDAPeerManager) GetColPeers() []peer.ID {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var peers []peer.ID
	if colMap, ok := pm.colPeers[pm.myPosition.Col]; ok {
		for peerID := range colMap {
			peers = append(peers, peerID)
		}
	}
	return peers
}

// GetSubnetPeers returns all connected peers in same row or column
func (pm *RDAPeerManager) GetSubnetPeers() []peer.ID {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peerSet := make(map[peer.ID]bool)

	// Add row peers
	if rowMap, ok := pm.rowPeers[pm.myPosition.Row]; ok {
		for peerID := range rowMap {
			peerSet[peerID] = true
		}
	}

	// Add column peers
	if colMap, ok := pm.colPeers[pm.myPosition.Col]; ok {
		for peerID := range colMap {
			peerSet[peerID] = true
		}
	}

	// Convert to slice
	var peers []peer.ID
	for peerID := range peerSet {
		peers = append(peers, peerID)
	}
	return peers
}

// GetMyPosition returns the grid position of this node
func (pm *RDAPeerManager) GetMyPosition() GridPosition {
	return pm.myPosition
}

// GetMyRow returns the row coordinate of this node
func (pm *RDAPeerManager) GetMyRow() int {
	return pm.myPosition.Row
}

// GetMyCol returns the column coordinate of this node
func (pm *RDAPeerManager) GetMyCol() int {
	return pm.myPosition.Col
}

// OnPeerConnected returns a channel that receives peer IDs when peers connect
func (pm *RDAPeerManager) OnPeerConnected() <-chan peer.ID {
	return pm.peerConnectedCh
}

// OnPeerDisconnected returns a channel that receives peer IDs when peers disconnect
func (pm *RDAPeerManager) OnPeerDisconnected() <-chan peer.ID {
	return pm.peerDisconnectedCh
}

// GetPeerPosition returns the grid position of a peer
func (pm *RDAPeerManager) GetPeerPosition(peerID peer.ID) (GridPosition, bool) {
	return pm.gridManager.GetPeerPosition(peerID)
}

// IsPeerInSameRow checks if a peer is in the same row
func (pm *RDAPeerManager) IsPeerInSameRow(peerID peer.ID) bool {
	pos, ok := pm.gridManager.GetPeerPosition(peerID)
	return ok && pos.Row == pm.myPosition.Row
}

// IsPeerInSameCol checks if a peer is in the same column
func (pm *RDAPeerManager) IsPeerInSameCol(peerID peer.ID) bool {
	pos, ok := pm.gridManager.GetPeerPosition(peerID)
	return ok && pos.Col == pm.myPosition.Col
}

// GetPeerCountInRow returns the number of peers in the same row
func (pm *RDAPeerManager) GetPeerCountInRow() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if rowMap, ok := pm.rowPeers[pm.myPosition.Row]; ok {
		return len(rowMap)
	}
	return 0
}

// GetPeerCountInCol returns the number of peers in the same column
func (pm *RDAPeerManager) GetPeerCountInCol() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if colMap, ok := pm.colPeers[pm.myPosition.Col]; ok {
		return len(colMap)
	}
	return 0
}

// GetStats returns statistics about the peer manager
func (pm *RDAPeerManager) GetStats() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return map[string]interface{}{
		"my_position":        pm.myPosition,
		"row_peers":          pm.GetPeerCountInRow(),
		"col_peers":          pm.GetPeerCountInCol(),
		"total_subnet_peers": len(pm.GetSubnetPeers()),
	}
}
