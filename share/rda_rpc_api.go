package share

import (
	"context"
)

// RDAAPI defines the RPC API interface for RDA operations
// This interface is used to expose RDA functionality over JSON-RPC
type RDAAPI interface {
	// GetMyPosition returns this node's position in the RDA grid
	GetMyPosition(ctx context.Context) (*RDAPosition, error)

	// GetStatus returns the current status of the RDA node
	GetStatus(ctx context.Context) (*RDAStatus, error)

	// GetNodeInfo returns detailed information about this RDA node
	GetNodeInfo(ctx context.Context) (*RDANodeInfo, error)

	// GetRowPeers returns all peers in the same row
	GetRowPeers(ctx context.Context) (*RDAPeerList, error)

	// GetColPeers returns all peers in the same column
	GetColPeers(ctx context.Context) (*RDAPeerList, error)

	// GetSubnetPeers returns all peers in the subnet (row + column)
	GetSubnetPeers(ctx context.Context) (*RDAPeerList, error)

	// GetGridDimensions returns the dimensions of the RDA grid
	GetGridDimensions(ctx context.Context) (*RDAGridInfo, error)

	// GetStats returns RDA operation statistics
	GetStats(ctx context.Context) (*RDAStats, error)

	// GetHealth returns health status of the RDA node
	GetHealth(ctx context.Context) (*RDAHealthStatus, error)
}

// rdaAPI implements the RDAAPI interface
type rdaAPI struct {
	service *RDANodeService
}

// NewRDAAPI creates a new RDA API instance
func NewRDAAPI(service *RDANodeService) RDAAPI {
	return &rdaAPI{
		service: service,
	}
}

// GetMyPosition returns this node's position in the grid
func (a *rdaAPI) GetMyPosition(ctx context.Context) (*RDAPosition, error) {
	pos := a.service.GetMyPosition()
	return &RDAPosition{
		Row: pos.Row,
		Col: pos.Col,
	}, nil
}

// GetStatus returns the current status of the node
func (a *rdaAPI) GetStatus(ctx context.Context) (*RDAStatus, error) {
	myPos := a.service.GetMyPosition()
	rowPeers := a.service.GetRowPeers()
	colPeers := a.service.GetColPeers()
	gridMgr := a.service.GetGridManager()

	dims := gridMgr.GetGridDimensions()
	rowID, colID := GetSubnetIDs(a.service.host.ID(), dims)

	return &RDAStatus{
		Position: RDAPosition{
			Row: myPos.Row,
			Col: myPos.Col,
		},
		RowPeers:   len(rowPeers),
		ColPeers:   len(colPeers),
		TotalPeers: len(rowPeers) + len(colPeers),
		GridDimensions: RDAGridInfo{
			Rows: int(dims.Rows),
			Cols: int(dims.Cols),
		},
		RowTopic: rowID,
		ColTopic: colID,
	}, nil
}

// GetNodeInfo returns detailed information about the node
func (a *rdaAPI) GetNodeInfo(ctx context.Context) (*RDANodeInfo, error) {
	myPos := a.service.GetMyPosition()
	gridMgr := a.service.GetGridManager()
	dims := gridMgr.GetGridDimensions()
	rowID, colID := GetSubnetIDs(a.service.host.ID(), dims)

	filter := a.service.GetPeerFilter()
	policy := filter.GetFilterPolicy()

	policyStr := "AllowAny"
	if policy.AllowAny {
		policyStr = "AllowAny"
	} else if policy.AllowRowCommunication && !policy.AllowColCommunication {
		policyStr = "RowOnly"
	} else if !policy.AllowRowCommunication && policy.AllowColCommunication {
		policyStr = "ColOnly"
	} else if policy.AllowRowCommunication && policy.AllowColCommunication {
		policyStr = "RowAndCol"
	}

	return &RDANodeInfo{
		PeerID: a.service.host.ID().String(),
		Position: RDAPosition{
			Row: myPos.Row,
			Col: myPos.Col,
		},
		GridDimensions: RDAGridInfo{
			Rows: int(dims.Rows),
			Cols: int(dims.Cols),
		},
		RowTopic:     rowID,
		ColTopic:     colID,
		FilterPolicy: policyStr,
	}, nil
}

// GetRowPeers returns all peers in the same row
func (a *rdaAPI) GetRowPeers(ctx context.Context) (*RDAPeerList, error) {
	peers := a.service.GetRowPeers()
	peerStrs := make([]string, len(peers))
	for i, p := range peers {
		peerStrs[i] = p.String()
	}
	return &RDAPeerList{
		Peers: peerStrs,
		Count: len(peerStrs),
	}, nil
}

// GetColPeers returns all peers in the same column
func (a *rdaAPI) GetColPeers(ctx context.Context) (*RDAPeerList, error) {
	peers := a.service.GetColPeers()
	peerStrs := make([]string, len(peers))
	for i, p := range peers {
		peerStrs[i] = p.String()
	}
	return &RDAPeerList{
		Peers: peerStrs,
		Count: len(peerStrs),
	}, nil
}

// GetSubnetPeers returns all peers in the subnet (row + column)
func (a *rdaAPI) GetSubnetPeers(ctx context.Context) (*RDAPeerList, error) {
	peers := a.service.GetSubnetPeers()
	peerStrs := make([]string, len(peers))
	for i, p := range peers {
		peerStrs[i] = p.String()
	}
	return &RDAPeerList{
		Peers: peerStrs,
		Count: len(peerStrs),
	}, nil
}

// GetGridDimensions returns the grid dimensions
func (a *rdaAPI) GetGridDimensions(ctx context.Context) (*RDAGridInfo, error) {
	gridMgr := a.service.GetGridManager()
	dims := gridMgr.GetGridDimensions()
	return &RDAGridInfo{
		Rows: int(dims.Rows),
		Cols: int(dims.Cols),
	}, nil
}

// GetStats returns RDA statistics
func (a *rdaAPI) GetStats(ctx context.Context) (*RDAStats, error) {
	router := a.service.GetGossipRouter()
	routerStats := router.GetStats()

	return &RDAStats{
		RowMessagesSent:   routerStats.RowMessagesSent,
		ColMessagesSent:   routerStats.ColMessagesSent,
		RowMessagesRecv:   routerStats.RowMessagesRecv,
		ColMessagesRecv:   routerStats.ColMessagesRecv,
		TotalMessageSent:  routerStats.RowMessagesSent + routerStats.ColMessagesSent,
		TotalMessagesRecv: routerStats.RowMessagesRecv + routerStats.ColMessagesRecv,
		PeersInRow:        routerStats.PeersInRow,
		PeersInCol:        routerStats.PeersInCol,
		TotalSubnetPeers:  routerStats.TotalSubnetPeers,
	}, nil
}

// GetHealth returns health status
func (a *rdaAPI) GetHealth(ctx context.Context) (*RDAHealthStatus, error) {
	rowPeers := a.service.GetRowPeers()
	colPeers := a.service.GetColPeers()
	totalPeers := len(rowPeers) + len(colPeers)

	gridMgr := a.service.GetGridManager()
	dims := gridMgr.GetGridDimensions()
	totalGridCells := int(dims.Rows) * int(dims.Cols)

	isHealthy := totalPeers > 0
	message := "healthy"
	if !isHealthy {
		message = "no peers connected"
	}

	coverageRate := float64(totalPeers) / float64(totalGridCells)

	return &RDAHealthStatus{
		IsHealthy:        isHealthy,
		Message:          message,
		ConnectedPeers:   totalPeers,
		GridCoverageRate: coverageRate,
	}, nil
}
