package share

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerFilter enforces RDA grid constraints on peer communication
type PeerFilter struct {
	gridManager  *RDAGridManager
	myPeerID     peer.ID
	myPosition   GridPosition
	filterPolicy FilterPolicy
	mu           sync.RWMutex
}

// FilterPolicy defines the peer filtering policy
type FilterPolicy struct {
	// AllowRowCommunication restricts peers to their row
	AllowRowCommunication bool
	// AllowColCommunication restricts peers to their column
	AllowColCommunication bool
	// AllowAny allows communication with any peer
	AllowAny bool
	// MaxRowPeers limits the number of row peers
	MaxRowPeers int
	// MaxColPeers limits the number of column peers
	MaxColPeers int
}

// DefaultFilterPolicy returns the default filtering policy
func DefaultFilterPolicy() FilterPolicy {
	return FilterPolicy{
		AllowRowCommunication: true,
		AllowColCommunication: true,
		AllowAny:              false,
		MaxRowPeers:           256,
		MaxColPeers:           256,
	}
}

// StrictFilterPolicy returns a strict filtering policy
func StrictFilterPolicy() FilterPolicy {
	return FilterPolicy{
		AllowRowCommunication: true,
		AllowColCommunication: true,
		AllowAny:              false,
		MaxRowPeers:           128,
		MaxColPeers:           128,
	}
}

// NewPeerFilter creates a new peer filter
func NewPeerFilter(
	host host.Host,
	gridManager *RDAGridManager,
	policy FilterPolicy,
) *PeerFilter {
	position := GridPosition{}
	if pos, ok := gridManager.GetPeerPosition(host.ID()); ok {
		position = pos
	}

	return &PeerFilter{
		gridManager:  gridManager,
		myPeerID:     host.ID(),
		myPosition:   position,
		filterPolicy: policy,
	}
}

// GetFilterPolicy returns the current filter policy
func (pf *PeerFilter) GetFilterPolicy() FilterPolicy {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.filterPolicy
}

// CanCommunicate checks if communication with a peer is allowed
func (pf *PeerFilter) CanCommunicate(peerID peer.ID) (bool, string) {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	if pf.filterPolicy.AllowAny {
		return true, "allow_any"
	}

	pos, ok := pf.gridManager.GetPeerPosition(peerID)
	if !ok {
		return false, "peer_not_registered"
	}

	// Check if peer is in the same row
	if pf.filterPolicy.AllowRowCommunication && pos.Row == pf.myPosition.Row {
		return true, "row_peer"
	}

	// Check if peer is in the same column
	if pf.filterPolicy.AllowColCommunication && pos.Col == pf.myPosition.Col {
		return true, "col_peer"
	}

	return false, "not_in_subnet"
}

// FilterPeers filters a list of peers based on the policy
func (pf *PeerFilter) FilterPeers(peers []peer.ID) []peer.ID {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	filtered := make([]peer.ID, 0, len(peers))
	rowCount := 0
	colCount := 0

	for _, p := range peers {
		can, reason := pf.CanCommunicate(p)
		if !can {
			continue
		}

		// Check limits
		if reason == "row_peer" {
			if pf.filterPolicy.MaxRowPeers > 0 && rowCount >= pf.filterPolicy.MaxRowPeers {
				continue
			}
			rowCount++
		} else if reason == "col_peer" {
			if pf.filterPolicy.MaxColPeers > 0 && colCount >= pf.filterPolicy.MaxColPeers {
				continue
			}
			colCount++
		}

		filtered = append(filtered, p)
	}

	return filtered
}

// GetAllowedPeers returns the list of all allowed peers based on the policy
func (pf *PeerFilter) GetAllowedPeers(gridManager *RDAGridManager) []peer.ID {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	allowed := make(map[peer.ID]bool)

	if pf.filterPolicy.AllowRowCommunication {
		for _, p := range gridManager.GetRowPeers(pf.myPosition.Row) {
			if p != pf.myPeerID {
				allowed[p] = true
			}
		}
	}

	if pf.filterPolicy.AllowColCommunication {
		for _, p := range gridManager.GetColPeers(pf.myPosition.Col) {
			if p != pf.myPeerID {
				allowed[p] = true
			}
		}
	}

	peers := make([]peer.ID, 0, len(allowed))
	for p := range allowed {
		peers = append(peers, p)
	}

	return peers
}

// SetPolicy updates the filtering policy
func (pf *PeerFilter) SetPolicy(policy FilterPolicy) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.filterPolicy = policy
}

// GetPolicy returns the current filtering policy
func (pf *PeerFilter) GetPolicy() FilterPolicy {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.filterPolicy
}

// GetStats returns statistics about the peer filter
func (pf *PeerFilter) GetStats(gridManager *RDAGridManager) map[string]interface{} {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	allowed := pf.GetAllowedPeers(gridManager)

	return map[string]interface{}{
		"my_position":       pf.myPosition,
		"allowed_peers":     len(allowed),
		"row_communication": pf.filterPolicy.AllowRowCommunication,
		"col_communication": pf.filterPolicy.AllowColCommunication,
		"allow_any":         pf.filterPolicy.AllowAny,
		"max_row_peers":     pf.filterPolicy.MaxRowPeers,
		"max_col_peers":     pf.filterPolicy.MaxColPeers,
	}
}

// ValidatePeerSubset validates that a peer subset matches the policy
func (pf *PeerFilter) ValidatePeerSubset(peers []peer.ID) error {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	for _, p := range peers {
		can, reason := pf.CanCommunicate(p)
		if !can {
			return fmt.Errorf("peer %s cannot communicate: %s", p.String()[:16], reason)
		}
	}
	return nil
}

// GetPeerSubnetInfo returns information about the peer's subnet
func (pf *PeerFilter) GetPeerSubnetInfo(peerID peer.ID) (map[string]interface{}, error) {
	can, reason := pf.CanCommunicate(peerID)

	pos, ok := pf.gridManager.GetPeerPosition(peerID)
	if !ok {
		return nil, fmt.Errorf("peer not registered in grid")
	}

	return map[string]interface{}{
		"peer_id":         peerID.String()[:16],
		"position":        pos,
		"can_communicate": can,
		"reason":          reason,
		"same_row":        pos.Row == pf.myPosition.Row,
		"same_col":        pos.Col == pf.myPosition.Col,
		"my_position":     pf.myPosition,
	}, nil
}

// MultiPeerFilter manages multiple peer filters for different policies
type MultiPeerFilter struct {
	filters map[string]*PeerFilter
	mu      sync.RWMutex
}

// NewMultiPeerFilter creates a new multi peer filter
func NewMultiPeerFilter() *MultiPeerFilter {
	return &MultiPeerFilter{
		filters: make(map[string]*PeerFilter),
	}
}

// RegisterFilter registers a named peer filter
func (mpf *MultiPeerFilter) RegisterFilter(name string, filter *PeerFilter) {
	mpf.mu.Lock()
	defer mpf.mu.Unlock()
	mpf.filters[name] = filter
}

// GetFilter returns a named peer filter
func (mpf *MultiPeerFilter) GetFilter(name string) (*PeerFilter, bool) {
	mpf.mu.RLock()
	defer mpf.mu.RUnlock()
	filter, ok := mpf.filters[name]
	return filter, ok
}

// CanCommunicateWith checks if communication is allowed using any filter
func (mpf *MultiPeerFilter) CanCommunicateWith(peerID peer.ID) bool {
	mpf.mu.RLock()
	defer mpf.mu.RUnlock()

	for _, filter := range mpf.filters {
		can, _ := filter.CanCommunicate(peerID)
		if can {
			return true
		}
	}

	return false
}
