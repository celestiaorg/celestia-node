package share

// RDAPosition represents a node's position in the RDA grid
type RDAPosition struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// RDAGridInfo contains information about the grid configuration
type RDAGridInfo struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

// RDAPeerInfo contains information about a peer
type RDAPeerInfo struct {
	PeerID    string      `json:"peer_id"`
	Position  RDAPosition `json:"position"`
	IsRowPeer bool        `json:"is_row_peer"`
	IsColPeer bool        `json:"is_col_peer"`
}

// RDAStatus contains the current status of the RDA node
type RDAStatus struct {
	// Node position
	Position RDAPosition `json:"position"`

	// Peer counts
	RowPeers   int `json:"row_peers"`
	ColPeers   int `json:"col_peers"`
	TotalPeers int `json:"total_peers"`

	// Grid information
	GridDimensions RDAGridInfo `json:"grid_dimensions"`

	// Network topics
	RowTopic string `json:"row_topic"`
	ColTopic string `json:"col_topic"`
}

// RDANodeInfo contains detailed information about the RDA node
type RDANodeInfo struct {
	PeerID         string      `json:"peer_id"`
	Position       RDAPosition `json:"position"`
	GridDimensions RDAGridInfo `json:"grid_dimensions"`
	RowTopic       string      `json:"row_topic"`
	ColTopic       string      `json:"col_topic"`
	FilterPolicy   string      `json:"filter_policy"`
}

// RDAStats contains statistics about RDA operations
type RDAStats struct {
	RowMessagesSent   uint64 `json:"row_messages_sent"`
	ColMessagesSent   uint64 `json:"col_messages_sent"`
	RowMessagesRecv   uint64 `json:"row_messages_recv"`
	ColMessagesRecv   uint64 `json:"col_messages_recv"`
	TotalMessageSent  uint64 `json:"total_messages_sent"`
	TotalMessagesRecv uint64 `json:"total_messages_recv"`
	PeersInRow        int    `json:"peers_in_row"`
	PeersInCol        int    `json:"peers_in_col"`
	TotalSubnetPeers  int    `json:"total_subnet_peers"`
}

// RDAPeerList contains a list of peers
type RDAPeerList struct {
	Peers []string `json:"peers"`
	Count int      `json:"count"`
}

// RDAHealthStatus contains health information
type RDAHealthStatus struct {
	IsHealthy        bool    `json:"is_healthy"`
	Message          string  `json:"message"`
	ConnectedPeers   int     `json:"connected_peers"`
	GridCoverageRate float64 `json:"grid_coverage_rate"`
}

// RDAPublishRequest contains parameters for publishing data
type RDAPublishRequest struct {
	// Data to publish (base64 encoded)
	Data string `json:"data"`
	// Optional tag to identify the data
	Tag string `json:"tag,omitempty"`
	// TTL in seconds
	TTL int `json:"ttl,omitempty"`
}

// RDAPublishResponse contains the result of publishing data
type RDAPublishResponse struct {
	// Success indicates if the publish operation succeeded
	Success bool `json:"success"`
	// DataHash is the hash of the published data
	DataHash string `json:"data_hash"`
	// Message provides additional information
	Message string `json:"message"`
	// PeersReached is the number of peers that received the data
	PeersReached int `json:"peers_reached"`
}

// RDADataRequest contains parameters for requesting data from grid peers
type RDADataRequest struct {
	// DataHash is the hash of the data being requested (hex encoded)
	DataHash string `json:"data_hash"`
	// Timeout in milliseconds
	Timeout int `json:"timeout,omitempty"`
	// Tag to filter responses
	Tag string `json:"tag,omitempty"`
}

// RDADataResponse contains the result of requesting data
type RDADataResponse struct {
	// Success indicates if the data was found
	Success bool `json:"success"`
	// Data is the retrieved data (base64 encoded)
	Data string `json:"data,omitempty"`
	// Source is the peer ID that provided the data
	Source string `json:"source,omitempty"`
	// Message provides additional information
	Message string `json:"message"`
	// ResponseTime in milliseconds
	ResponseTime int `json:"response_time"`
}

// RDABatchDataResponse contains responses from multiple peers
type RDABatchDataResponse struct {
	// RequestID is the correlation ID for this batch request
	RequestID string `json:"request_id"`
	// Found indicates if data was found
	Found bool `json:"found"`
	// Message provides additional information
	Message string `json:"message"`
	// Responses from different peers
	Responses []RDADataResponse `json:"responses"`
	// PeersQueried is the number of peers that were queried
	PeersQueried int `json:"peers_queried"`
}

// RDAPeerEvent represents a peer connection/disconnection event
type RDAPeerEvent struct {
	// PeerID is the ID of the peer
	PeerID string `json:"peer_id"`
	// EventType: "connected" or "disconnected"
	EventType string `json:"event_type"`
	// Timestamp in Unix milliseconds
	Timestamp int64 `json:"timestamp"`
	// IsRowPeer indicates if peer is in the same row
	IsRowPeer bool `json:"is_row_peer"`
	// IsColPeer indicates if peer is in the same column
	IsColPeer bool `json:"is_col_peer"`
}

// RDASubnetMember represents a discovered member in a subnet
type RDASubnetMember struct {
	PeerID    string   `json:"peer_id"`
	Addresses []string `json:"addresses"`
	LastSeen  int64    `json:"last_seen"` // Unix nano
}

// RDASubnetMembersResponse contains subnet membership information
type RDASubnetMembersResponse struct {
	RowMembers int               `json:"row_members"`
	ColMembers int               `json:"col_members"`
	Members    []RDASubnetMember `json:"members"`
	Message    string            `json:"message"`
}

// RDAAnnouncementResponse contains the result of an announcement
type RDAAnnouncementResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	RowSubnetName     string `json:"row_subnet_name"`
	ColSubnetName     string `json:"col_subnet_name"`
	AnnouncedAt       int64  `json:"announced_at"`       // Unix nano
	MembersDiscovered int    `json:"members_discovered"` // After delay pull
}
