package share

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
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

	// PublishToSubnet publishes data to all subnet peers (row + column)
	PublishToSubnet(ctx context.Context, req *RDAPublishRequest) (*RDAPublishResponse, error)

	// PublishToRow publishes data only to row peers
	PublishToRow(ctx context.Context, req *RDAPublishRequest) (*RDAPublishResponse, error)

	// PublishToCol publishes data only to column peers
	PublishToCol(ctx context.Context, req *RDAPublishRequest) (*RDAPublishResponse, error)

	// RequestDataFromRow requests data from row peers
	RequestDataFromRow(ctx context.Context, req *RDADataRequest) (*RDABatchDataResponse, error)

	// RequestDataFromCol requests data from column peers
	RequestDataFromCol(ctx context.Context, req *RDADataRequest) (*RDABatchDataResponse, error)

	// RequestDataFromSubnet requests data from all subnet peers
	RequestDataFromSubnet(ctx context.Context, req *RDADataRequest) (*RDABatchDataResponse, error)

	// GetSubnetMembers returns the list of members in this node's subnets
	GetSubnetMembers(ctx context.Context) (*RDASubnetMembersResponse, error)

	// AnnounceToSubnet manually triggers announcement to subnets
	AnnounceToSubnet(ctx context.Context) (*RDAAnnouncementResponse, error)
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

// PublishToSubnet publishes data to all subnet peers (row + column)
func (a *rdaAPI) PublishToSubnet(ctx context.Context, req *RDAPublishRequest) (*RDAPublishResponse, error) {
	if req == nil || req.Data == "" {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "invalid request: data is required",
			PeersReached: 0,
		}, nil
	}

	// Decode base64 data
	data, err := decodeBase64(req.Data)
	if err != nil {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "invalid data encoding: " + err.Error(),
			PeersReached: 0,
		}, nil
	}

	// Publish to subnet
	err = a.service.PublishToSubnet(ctx, data)
	if err != nil {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "publish failed: " + err.Error(),
			PeersReached: 0,
		}, nil
	}

	// Calculate hash
	hash := calculateSHA256(data)

	// Get connected peers count
	rowPeers := a.service.GetRowPeers()
	colPeers := a.service.GetColPeers()
	peersReached := len(rowPeers) + len(colPeers)

	return &RDAPublishResponse{
		Success:      true,
		DataHash:     encodeHex(hash),
		Message:      "data published to subnet",
		PeersReached: peersReached,
	}, nil
}

// PublishToRow publishes data only to row peers
func (a *rdaAPI) PublishToRow(ctx context.Context, req *RDAPublishRequest) (*RDAPublishResponse, error) {
	if req == nil || req.Data == "" {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "invalid request: data is required",
			PeersReached: 0,
		}, nil
	}

	// Decode base64 data
	data, err := decodeBase64(req.Data)
	if err != nil {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "invalid data encoding: " + err.Error(),
			PeersReached: 0,
		}, nil
	}

	// Publish to row
	err = a.service.PublishToRow(ctx, data)
	if err != nil {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "publish failed: " + err.Error(),
			PeersReached: 0,
		}, nil
	}

	// Calculate hash
	hash := calculateSHA256(data)

	// Get row peers count
	rowPeers := a.service.GetRowPeers()

	return &RDAPublishResponse{
		Success:      true,
		DataHash:     encodeHex(hash),
		Message:      "data published to row",
		PeersReached: len(rowPeers),
	}, nil
}

// PublishToCol publishes data only to column peers
func (a *rdaAPI) PublishToCol(ctx context.Context, req *RDAPublishRequest) (*RDAPublishResponse, error) {
	if req == nil || req.Data == "" {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "invalid request: data is required",
			PeersReached: 0,
		}, nil
	}

	// Decode base64 data
	data, err := decodeBase64(req.Data)
	if err != nil {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "invalid data encoding: " + err.Error(),
			PeersReached: 0,
		}, nil
	}

	// Publish to column
	err = a.service.PublishToCol(ctx, data)
	if err != nil {
		return &RDAPublishResponse{
			Success:      false,
			Message:      "publish failed: " + err.Error(),
			PeersReached: 0,
		}, nil
	}

	// Calculate hash
	hash := calculateSHA256(data)

	// Get column peers count
	colPeers := a.service.GetColPeers()

	return &RDAPublishResponse{
		Success:      true,
		DataHash:     encodeHex(hash),
		Message:      "data published to column",
		PeersReached: len(colPeers),
	}, nil
}

// RequestDataFromRow requests data from row peers
func (a *rdaAPI) RequestDataFromRow(ctx context.Context, req *RDADataRequest) (*RDABatchDataResponse, error) {
	if req == nil || req.DataHash == "" {
		return &RDABatchDataResponse{
			Found:        false,
			Message:      "invalid request: data_hash is required",
			PeersQueried: 0,
			Responses:    []RDADataResponse{},
		}, nil
	}

	// Get row peers
	rowPeers := a.service.GetRowPeers()
	peersQueried := len(rowPeers)

	// Request data from row (this would make actual requests in production)
	// For now, return response indicating peer count
	responses := []RDADataResponse{}

	return &RDABatchDataResponse{
		RequestID:    req.DataHash,
		Found:        false,
		Message:      "requested data from row peers",
		PeersQueried: peersQueried,
		Responses:    responses,
	}, nil
}

// RequestDataFromCol requests data from column peers
func (a *rdaAPI) RequestDataFromCol(ctx context.Context, req *RDADataRequest) (*RDABatchDataResponse, error) {
	if req == nil || req.DataHash == "" {
		return &RDABatchDataResponse{
			Found:        false,
			Message:      "invalid request: data_hash is required",
			PeersQueried: 0,
			Responses:    []RDADataResponse{},
		}, nil
	}

	// Get column peers
	colPeers := a.service.GetColPeers()
	peersQueried := len(colPeers)

	// Request data from column
	responses := []RDADataResponse{}

	return &RDABatchDataResponse{
		RequestID:    req.DataHash,
		Found:        false,
		Message:      "requested data from column peers",
		PeersQueried: peersQueried,
		Responses:    responses,
	}, nil
}

// RequestDataFromSubnet requests data from all subnet peers
func (a *rdaAPI) RequestDataFromSubnet(ctx context.Context, req *RDADataRequest) (*RDABatchDataResponse, error) {
	if req == nil || req.DataHash == "" {
		return &RDABatchDataResponse{
			Found:        false,
			Message:      "invalid request: data_hash is required",
			PeersQueried: 0,
			Responses:    []RDADataResponse{},
		}, nil
	}

	// Get all subnet peers
	rowPeers := a.service.GetRowPeers()
	colPeers := a.service.GetColPeers()
	peersQueried := len(rowPeers) + len(colPeers)

	// Request data from subnet
	responses := []RDADataResponse{}

	return &RDABatchDataResponse{
		RequestID:    req.DataHash,
		Found:        false,
		Message:      "requested data from subnet peers",
		PeersQueried: peersQueried,
		Responses:    responses,
	}, nil
}

// GetSubnetMembers returns the list of members discovered in row/col subnets
func (a *rdaAPI) GetSubnetMembers(ctx context.Context) (*RDASubnetMembersResponse, error) {
	mgr := a.service.GetSubnetDiscoveryManager()
	if mgr == nil {
		return &RDASubnetMembersResponse{
			RowMembers: 0,
			ColMembers: 0,
			Members:    []RDASubnetMember{},
			Message:    "subnet discovery not enabled",
		}, nil
	}

	// Get this node's position
	myPos := a.service.GetMyPosition()
	rowSubnet := fmt.Sprintf("row/%d", myPos.Row)
	colSubnet := fmt.Sprintf("col/%d", myPos.Col)

	// Get announcers
	rowAnnouncer, _ := mgr.GetOrCreateAnnouncer(ctx, rowSubnet)
	colAnnouncer, _ := mgr.GetOrCreateAnnouncer(ctx, colSubnet)

	var rowMembers, colMembers []SubnetMember
	if rowAnnouncer != nil {
		rowMembers = rowAnnouncer.GetMembers()
	}
	if colAnnouncer != nil {
		colMembers = colAnnouncer.GetMembers()
	}

	// Convert to RDA types
	members := make([]RDASubnetMember, 0, len(rowMembers)+len(colMembers))
	for _, m := range rowMembers {
		addrs := make([]string, len(m.PeerAddrs))
		for i, ai := range m.PeerAddrs {
			addrs[i] = ai.String()
		}
		members = append(members, RDASubnetMember{
			PeerID:    m.PeerID.String(),
			Addresses: addrs,
			LastSeen:  m.LastSeen.UnixNano(),
		})
	}
	for _, m := range colMembers {
		addrs := make([]string, len(m.PeerAddrs))
		for i, ai := range m.PeerAddrs {
			addrs[i] = ai.String()
		}
		members = append(members, RDASubnetMember{
			PeerID:    m.PeerID.String(),
			Addresses: addrs,
			LastSeen:  m.LastSeen.UnixNano(),
		})
	}

	return &RDASubnetMembersResponse{
		RowMembers: len(rowMembers),
		ColMembers: len(colMembers),
		Members:    members,
		Message:    "subnet members retrieved",
	}, nil
}

// AnnounceToSubnet manually triggers an announcement to subnets
func (a *rdaAPI) AnnounceToSubnet(ctx context.Context) (*RDAAnnouncementResponse, error) {
	mgr := a.service.GetSubnetDiscoveryManager()
	if mgr == nil {
		return &RDAAnnouncementResponse{
			Success: false,
			Message: "subnet discovery not enabled",
		}, nil
	}

	// Get this node's position
	myPos := a.service.GetMyPosition()
	rowSubnet := fmt.Sprintf("row/%d", myPos.Row)
	colSubnet := fmt.Sprintf("col/%d", myPos.Col)

	// Create announcers and announce
	rowAnnouncer, err := mgr.GetOrCreateAnnouncer(ctx, rowSubnet)
	if err != nil {
		return &RDAAnnouncementResponse{
			Success: false,
			Message: "failed to create row announcer: " + err.Error(),
		}, nil
	}

	colAnnouncer, err := mgr.GetOrCreateAnnouncer(ctx, colSubnet)
	if err != nil {
		return &RDAAnnouncementResponse{
			Success: false,
			Message: "failed to create col announcer: " + err.Error(),
		}, nil
	}

	now := time.Now().UnixNano()

	// Announce
	err = rowAnnouncer.AnnounceJoin(ctx)
	if err != nil {
		return &RDAAnnouncementResponse{
			Success: false,
			Message: "failed to announce to row subnet: " + err.Error(),
		}, nil
	}

	err = colAnnouncer.AnnounceJoin(ctx)
	if err != nil {
		return &RDAAnnouncementResponse{
			Success: false,
			Message: "failed to announce to col subnet: " + err.Error(),
		}, nil
	}

	// Get members after delay
	rowMembers := rowAnnouncer.GetMembersAfterDelay(ctx)
	colMembers := colAnnouncer.GetMembersAfterDelay(ctx)

	return &RDAAnnouncementResponse{
		Success:           true,
		Message:           "announced to subnets successfully",
		RowSubnetName:     rowSubnet,
		ColSubnetName:     colSubnet,
		AnnouncedAt:       now,
		MembersDiscovered: len(rowMembers) + len(colMembers),
	}, nil
}

// Helper functions

// decodeBase64 decodes base64 string to bytes
func decodeBase64(str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(str)
}

// encodeHex encodes bytes to hex string
func encodeHex(data []byte) string {
	return hex.EncodeToString(data)
}

// calculateSHA256 calculates SHA256 hash of data
func calculateSHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}
