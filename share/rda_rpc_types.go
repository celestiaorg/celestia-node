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
	PeerID   string       `json:"peer_id"`
	Position RDAPosition  `json:"position"`
	IsRowPeer bool        `json:"is_row_peer"`
	IsColPeer bool        `json:"is_col_peer"`
}

// RDAStatus contains the current status of the RDA node
type RDAStatus struct {
	// Node position
	Position RDAPosition `json:"position"`

	// Peer counts
	RowPeers       int `json:"row_peers"`
	ColPeers       int `json:"col_peers"`
	TotalPeers     int `json:"total_peers"`

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
	RowMessagesSent  uint64 `json:"row_messages_sent"`
	ColMessagesSent  uint64 `json:"col_messages_sent"`
	RowMessagesRecv  uint64 `json:"row_messages_recv"`
	ColMessagesRecv  uint64 `json:"col_messages_recv"`
	TotalMessageSent uint64 `json:"total_messages_sent"`
	TotalMessagesRecv uint64 `json:"total_messages_recv"`
	PeersInRow       int    `json:"peers_in_row"`
	PeersInCol       int    `json:"peers_in_col"`
	TotalSubnetPeers int    `json:"total_subnet_peers"`
}

// RDAPeerList contains a list of peers
type RDAPeerList struct {
	Peers []string `json:"peers"`
	Count int      `json:"count"`
}

// RDAHealthStatus contains health information
type RDAHealthStatus struct {
	IsHealthy        bool   `json:"is_healthy"`
	Message          string `json:"message"`
	ConnectedPeers   int    `json:"connected_peers"`
	GridCoverageRate float64 `json:"grid_coverage_rate"`
}
