package share

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RDAGossipSubRouter implements custom gossip routing for RDA grid topology
type RDAGossipSubRouter struct {
	gridManager   *RDAGridManager
	peerManager   *RDAPeerManager
	subnetManager *RDASubnetManager
	peerFilter    *PeerFilter
	myPeerID      peer.ID
	myPosition    GridPosition

	// Statistics
	stats *RDARouterStats

	mu sync.RWMutex
}

// RDARouterStats tracks routing statistics
type RDARouterStats struct {
	RowMessagesSent  uint64
	ColMessagesSent  uint64
	RowMessagesRecv  uint64
	ColMessagesRecv  uint64
	PeersInRow       int
	PeersInCol       int
	TotalSubnetPeers int
}

// NewRDAGossipSubRouter creates a new RDA gossip router
func NewRDAGossipSubRouter(
	host host.Host,
	gridManager *RDAGridManager,
	peerManager *RDAPeerManager,
	subnetManager *RDASubnetManager,
	peerFilter *PeerFilter,
) *RDAGossipSubRouter {
	position, _ := gridManager.GetPeerPosition(host.ID())

	return &RDAGossipSubRouter{
		gridManager:   gridManager,
		peerManager:   peerManager,
		subnetManager: subnetManager,
		peerFilter:    peerFilter,
		myPeerID:      host.ID(),
		myPosition:    position,
		stats:         &RDARouterStats{},
	}
}

// RouteToRow routes a message to row peers
func (r *RDAGossipSubRouter) RouteToRow(ctx context.Context, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.subnetManager.PublishToRow(ctx, data); err != nil {
		log.Warnf("failed to route to row: %v", err)
		return err
	}

	r.stats.RowMessagesSent++
	return nil
}

// RouteToCol routes a message to column peers
func (r *RDAGossipSubRouter) RouteToCol(ctx context.Context, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.subnetManager.PublishToCol(ctx, data); err != nil {
		log.Warnf("failed to route to col: %v", err)
		return err
	}

	r.stats.ColMessagesSent++
	return nil
}

// RouteToSubnet routes a message to all subnet peers (row + col)
func (r *RDAGossipSubRouter) RouteToSubnet(ctx context.Context, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.subnetManager.PublishToSubnet(ctx, data); err != nil {
		log.Warnf("failed to route to subnet: %v", err)
		return err
	}

	r.stats.RowMessagesSent++
	r.stats.ColMessagesSent++
	return nil
}

// SelectPeersForMessage selects peers to forward a message to based on RDA topology
func (r *RDAGossipSubRouter) SelectPeersForMessage(
	allPeers []peer.ID,
	excludePeer peer.ID,
) []peer.ID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Filter peers that are in the same subnet
	subnetPeers := make([]peer.ID, 0)
	for _, p := range allPeers {
		if p == excludePeer {
			continue
		}

		can, _ := r.peerFilter.CanCommunicate(p)
		if can {
			subnetPeers = append(subnetPeers, p)
		}
	}

	return subnetPeers
}

// GetRowPeers returns peers in the same row
func (r *RDAGossipSubRouter) GetRowPeers() []peer.ID {
	return r.peerManager.GetRowPeers()
}

// GetColPeers returns peers in the same column
func (r *RDAGossipSubRouter) GetColPeers() []peer.ID {
	return r.peerManager.GetColPeers()
}

// GetSubnetPeers returns all peers in row and column
func (r *RDAGossipSubRouter) GetSubnetPeers() []peer.ID {
	return r.peerManager.GetSubnetPeers()
}

// GetStats returns routing statistics
func (r *RDAGossipSubRouter) GetStats() RDARouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := *r.stats
	stats.PeersInRow = len(r.GetRowPeers())
	stats.PeersInCol = len(r.GetColPeers())
	stats.TotalSubnetPeers = len(r.GetSubnetPeers())

	return stats
}

// ResetStats resets the statistics
func (r *RDAGossipSubRouter) ResetStats() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats = &RDARouterStats{}
}

// ValidatePeerSet validates that a set of peers is valid for RDA topology
func (r *RDAGossipSubRouter) ValidatePeerSet(peers []peer.ID) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.peerFilter.ValidatePeerSubset(peers)
}

// RDAExchangeCoordinator coordinates data exchange between row and column peers
type RDAExchangeCoordinator struct {
	gridManager   *RDAGridManager
	peerManager   *RDAPeerManager
	subnetManager *RDASubnetManager
	peerFilter    *PeerFilter
	myPeerID      peer.ID
	myPosition    GridPosition

	// Exchange strategies for rows and columns
	rowExchangeCh chan exchangeTask
	colExchangeCh chan exchangeTask

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type exchangeTask struct {
	dataHash []byte
	peers    []peer.ID
	callback chan exchangeResult
}

type exchangeResult struct {
	success bool
	err     error
	data    []byte
}

// NewRDAExchangeCoordinator creates a new RDA exchange coordinator
func NewRDAExchangeCoordinator(
	host host.Host,
	gridManager *RDAGridManager,
	peerManager *RDAPeerManager,
	subnetManager *RDASubnetManager,
	peerFilter *PeerFilter,
) *RDAExchangeCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	position, _ := gridManager.GetPeerPosition(host.ID())

	return &RDAExchangeCoordinator{
		gridManager:   gridManager,
		peerManager:   peerManager,
		subnetManager: subnetManager,
		peerFilter:    peerFilter,
		myPeerID:      host.ID(),
		myPosition:    position,
		rowExchangeCh: make(chan exchangeTask, 100),
		colExchangeCh: make(chan exchangeTask, 100),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the exchange coordinator
func (c *RDAExchangeCoordinator) Start() error {
	c.wg.Add(2)
	go c.rowExchangeWorker()
	go c.colExchangeWorker()
	log.Infof("RDA exchange coordinator started")
	return nil
}

// Stop stops the exchange coordinator
func (c *RDAExchangeCoordinator) Stop() error {
	c.cancel()
	c.wg.Wait()
	close(c.rowExchangeCh)
	close(c.colExchangeCh)
	log.Infof("RDA exchange coordinator stopped")
	return nil
}

// RequestFromRow requests data from row peers
func (c *RDAExchangeCoordinator) RequestFromRow(dataHash []byte) <-chan exchangeResult {
	resultCh := make(chan exchangeResult, 1)
	task := exchangeTask{
		dataHash: dataHash,
		peers:    c.peerManager.GetRowPeers(),
		callback: resultCh,
	}

	select {
	case c.rowExchangeCh <- task:
	case <-c.ctx.Done():
		resultCh <- exchangeResult{success: false, err: context.Canceled}
		close(resultCh)
	}

	return resultCh
}

// RequestFromCol requests data from column peers
func (c *RDAExchangeCoordinator) RequestFromCol(dataHash []byte) <-chan exchangeResult {
	resultCh := make(chan exchangeResult, 1)
	task := exchangeTask{
		dataHash: dataHash,
		peers:    c.peerManager.GetColPeers(),
		callback: resultCh,
	}

	select {
	case c.colExchangeCh <- task:
	case <-c.ctx.Done():
		resultCh <- exchangeResult{success: false, err: context.Canceled}
		close(resultCh)
	}

	return resultCh
}

// rowExchangeWorker processes row exchange requests
func (c *RDAExchangeCoordinator) rowExchangeWorker() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case task := <-c.rowExchangeCh:
			// Process row exchange
			log.Debugf("Processing row exchange for hash %x", task.dataHash[:8])

			if len(task.peers) == 0 {
				task.callback <- exchangeResult{
					success: false,
					err:     fmt.Errorf("no row peers available"),
				}
				close(task.callback)
				continue
			}

			// Try to get data from row peers
			c.processExchangeTask(task)
		}
	}
}

// colExchangeWorker processes column exchange requests
func (c *RDAExchangeCoordinator) colExchangeWorker() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case task := <-c.colExchangeCh:
			// Process column exchange
			log.Debugf("Processing col exchange for hash %x", task.dataHash[:8])

			if len(task.peers) == 0 {
				task.callback <- exchangeResult{
					success: false,
					err:     fmt.Errorf("no col peers available"),
				}
				close(task.callback)
				continue
			}

			// Try to get data from column peers
			c.processExchangeTask(task)
		}
	}
}

// processExchangeTask processes an exchange task
func (c *RDAExchangeCoordinator) processExchangeTask(task exchangeTask) {
	// This is a placeholder for actual data exchange logic
	// In real implementation, this would:
	// 1. Create bitswap/graphsync requests
	// 2. Try to fetch from task.peers in parallel
	// 3. Return first successful result

	log.Debugf("Exchange task: attempting to get data from %d peers", len(task.peers))

	// Simulate successful exchange
	task.callback <- exchangeResult{
		success: true,
		err:     nil,
		data:    []byte("exchange_data"),
	}
	close(task.callback)
}

// GetMyGridPosition returns the node's position in the grid
func (c *RDAExchangeCoordinator) GetMyGridPosition() GridPosition {
	return c.myPosition
}

// CanExchangeWith checks if exchange is allowed with a peer
func (c *RDAExchangeCoordinator) CanExchangeWith(peerID peer.ID) bool {
	can, _ := c.peerFilter.CanCommunicate(peerID)
	return can
}
