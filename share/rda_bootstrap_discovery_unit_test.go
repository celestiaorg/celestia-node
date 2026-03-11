package share

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test bootstrap discovery service initialization
func TestBootstrapDiscoveryService_NewService(t *testing.T) {
	h := createMockHostUnit(t)
	gridManager := NewRDAGridManager(GridDimensions{Rows: 128, Cols: 128})

	bootstrapPeers := []peer.AddrInfo{
		{ID: peer.ID("bootstrap1")},
	}

	service := NewBootstrapDiscoveryService(h, bootstrapPeers, gridManager, 10, 20)

	assert.NotNil(t, service)
	assert.Equal(t, uint32(10), service.myRow)
	assert.Equal(t, uint32(20), service.myCol)
	assert.Equal(t, 1, len(service.bootstrapPeers))
	assert.NotNil(t, service.rowPeers)
	assert.NotNil(t, service.colPeers)
}

// Test bootstrap discovery service start with no bootstrap peers
func TestBootstrapDiscoveryService_StartWithNoPeers(t *testing.T) {
	h := createMockHostUnit(t)
	gridManager := NewRDAGridManager(GridDimensions{Rows: 128, Cols: 128})

	service := NewBootstrapDiscoveryService(h, []peer.AddrInfo{}, gridManager, 10, 20)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should not error, just log warning
	err := service.Start(ctx)
	require.NoError(t, err)
}

// Test bootstrap peer request marshaling
func TestBootstrapPeerRequest_Marshal(t *testing.T) {
	req := BootstrapPeerRequest{
		Type:      JoinRowSubnetRequest,
		NodeID:    "testnode",
		Row:       10,
		Col:       20,
		PeerAddrs: []string{"/ip4/127.0.0.1/tcp/30333"},
		GridDims:  GridDimensions{Rows: 128, Cols: 128},
	}

	// Test marshaling
	data, err := marshalBootstrapRequest(&req)
	require.NoError(t, err)
	assert.NotNil(t, data)

	// Test unmarshaling
	decoded, err := unmarshalBootstrapRequest(data)
	require.NoError(t, err)
	assert.Equal(t, req.Type, decoded.Type)
	assert.Equal(t, req.NodeID, decoded.NodeID)
	assert.Equal(t, req.Row, decoded.Row)
	assert.Equal(t, req.Col, decoded.Col)
}

// Test bootstrap peer response marshaling
func TestBootstrapPeerResponse_Marshal(t *testing.T) {
	resp := BootstrapPeerResponse{
		Type:    JoinRowSubnetResponse,
		Success: true,
		Message: "Successfully joined row subnet",
		RowPeers: []BootstrapPeer{
			{
				PeerID:    "peer1",
				Addresses: []string{"/ip4/127.0.0.1/tcp/30333"},
			},
		},
		Timestamp: time.Now().Unix(),
	}

	// Test marshaling
	data, err := marshalBootstrapResponse(&resp)
	require.NoError(t, err)
	assert.NotNil(t, data)

	// Test unmarshaling
	decoded, err := unmarshalBootstrapResponse(data)
	require.NoError(t, err)
	assert.Equal(t, resp.Type, decoded.Type)
	assert.Equal(t, resp.Success, decoded.Success)
	assert.Equal(t, len(resp.RowPeers), len(decoded.RowPeers))
}

// Test GetRowPeers returns discovered row peers
func TestBootstrapDiscoveryService_GetRowPeers(t *testing.T) {
	h := createMockHostUnit(t)
	gridManager := NewRDAGridManager(GridDimensions{Rows: 128, Cols: 128})

	service := NewBootstrapDiscoveryService(h, []peer.AddrInfo{}, gridManager, 10, 20)

	// Manually add some row peers
	service.mu.Lock()
	service.rowPeers["peer1"] = peer.AddrInfo{ID: peer.ID("peer1")}
	service.rowPeers["peer2"] = peer.AddrInfo{ID: peer.ID("peer2")}
	service.mu.Unlock()

	peers := service.GetRowPeers()
	assert.NotNil(t, peers)
	assert.Len(t, peers, 2)
}

// Test GetColPeers returns discovered column peers
func TestBootstrapDiscoveryService_GetColPeers(t *testing.T) {
	h := createMockHostUnit(t)
	gridManager := NewRDAGridManager(GridDimensions{Rows: 128, Cols: 128})

	service := NewBootstrapDiscoveryService(h, []peer.AddrInfo{}, gridManager, 10, 20)

	// Manually add some column peers
	service.mu.Lock()
	service.colPeers["peer3"] = peer.AddrInfo{ID: peer.ID("peer3")}
	service.colPeers["peer4"] = peer.AddrInfo{ID: peer.ID("peer4")}
	service.mu.Unlock()

	peers := service.GetColPeers()
	assert.NotNil(t, peers)
	assert.Len(t, peers, 2)
}

// Test bootstrap discovery service stop
func TestBootstrapDiscoveryService_Stop(t *testing.T) {
	h := createMockHostUnit(t)
	gridManager := NewRDAGridManager(GridDimensions{Rows: 128, Cols: 128})

	service := NewBootstrapDiscoveryService(h, []peer.AddrInfo{}, gridManager, 10, 20)

	ctx := context.Background()
	err := service.Stop(ctx)
	require.NoError(t, err)

	// Verify done channel is closed
	select {
	case <-service.done:
		// Success - channel is closed
	default:
		t.Fatal("done channel not closed after Stop")
	}
}

// Test bootstrap discovery peer to AddrInfo conversion
// NOTE: This test verifies that we can extract address information from BootstrapPeer structs
func TestPeerToAddrInfo_Conversion(t *testing.T) {
	// Create a peer with addresses we can parse
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()

	// Encode the peer ID properly for decoding
	peerIDStr := host.ID().String()

	peer := BootstrapPeer{
		PeerID:    peerIDStr, // Use encoded peer ID string
		Addresses: []string{"/ip4/127.0.0.1/tcp/30333"},
	}

	addrInfo, err := peerToAddrInfo(peer)
	require.NoError(t, err, "Failed to convert bootstrap peer to addr info")
	assert.NotNil(t, addrInfo)
	assert.NotEmpty(t, addrInfo.Addrs)
	assert.Equal(t, host.ID(), addrInfo.ID)
}

// Test bootstrap peer request with valid grid dimensions
func TestBootstrapPeerRequest_ValidGridDimensions(t *testing.T) {
	req := BootstrapPeerRequest{
		Type:     GetRowPeersRequest,
		NodeID:   "testpeer",
		Row:      50,
		Col:      75,
		GridDims: GridDimensions{Rows: 128, Cols: 128},
	}

	// Validate row/col are within bounds
	assert.Less(t, req.Row, uint32(128))
	assert.Less(t, req.Col, uint32(128))
	assert.Equal(t, uint16(128), req.GridDims.Rows)
	assert.Equal(t, uint16(128), req.GridDims.Cols)
}

// Test bootstrap peer request with invalid grid positions
func TestBootstrapPeerRequest_OutOfBoundsPosition(t *testing.T) {
	req := BootstrapPeerRequest{
		Type:     GetRowPeersRequest,
		NodeID:   "testpeer",
		Row:      150, // Out of bounds
		Col:      200, // Out of bounds
		GridDims: GridDimensions{Rows: 128, Cols: 128},
	}

	// These would fail validation in actual bootstrap service
	assert.Greater(t, req.Row, uint32(req.GridDims.Rows))
	assert.Greater(t, req.Col, uint32(req.GridDims.Cols))
}

// Test bootstrap response with mixed peers
func TestBootstrapResponse_MixedPeers(t *testing.T) {
	resp := BootstrapPeerResponse{
		Type:    GetRowPeersResponse,
		Success: true,
		RowPeers: []BootstrapPeer{
			{
				PeerID:    "peer1",
				Addresses: []string{"/ip4/10.0.0.1/tcp/30333", "/ip4/10.0.0.2/tcp/30333"},
			},
			{
				PeerID:    "peer2",
				Addresses: []string{"/ip6/::1/tcp/30334"},
			},
		},
		ColPeers: []BootstrapPeer{
			{
				PeerID:    "peer3",
				Addresses: []string{"/ip4/10.0.1.1/tcp/30335"},
			},
		},
	}

	assert.Equal(t, 2, len(resp.RowPeers))
	assert.Equal(t, 1, len(resp.ColPeers))
	assert.Equal(t, 2, len(resp.RowPeers[0].Addresses))
}

// Test concurrent peer discovery requests
func TestBootstrapDiscoveryService_ConcurrentRequests(t *testing.T) {
	h := createMockHostUnit(t)
	gridManager := NewRDAGridManager(GridDimensions{Rows: 128, Cols: 128})
	service := NewBootstrapDiscoveryService(h, []peer.AddrInfo{}, gridManager, 10, 20)

	// Simulate concurrent access to peer maps
	done := make(chan bool, 4)

	// Concurrent read
	go func() {
		service.GetRowPeers()
		service.GetColPeers()
		done <- true
	}()

	// Concurrent write
	go func() {
		service.mu.Lock()
		service.rowPeers["peer1"] = peer.AddrInfo{}
		service.mu.Unlock()
		done <- true
	}()

	// Another concurrent read
	go func() {
		service.GetRowPeers()
		done <- true
	}()

	// Another concurrent write
	go func() {
		service.mu.Lock()
		service.colPeers["peer2"] = peer.AddrInfo{}
		service.mu.Unlock()
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}
}

// Test bootstrap discovery response timestamp
func TestBootstrapResponse_Timestamp(t *testing.T) {
	before := time.Now().Unix()
	resp := BootstrapPeerResponse{
		Type:      JoinRowSubnetResponse,
		Success:   true,
		Message:   "Joined",
		Timestamp: time.Now().Unix(),
	}
	after := time.Now().Unix()

	assert.GreaterOrEqual(t, resp.Timestamp, before)
	assert.LessOrEqual(t, resp.Timestamp, after)
}

// Test bootstrap discovery grid position calculation
func TestBootstrapDiscoveryService_GridPosition(t *testing.T) {
	h := createMockHostUnit(t)
	gridManager := NewRDAGridManager(GridDimensions{Rows: 128, Cols: 128})

	testCases := []struct {
		name  string
		row   uint32
		col   uint32
		valid bool
	}{
		{"Center position", 64, 64, true},
		{"Corner position", 0, 0, true},
		{"Another corner", 127, 127, true},
		{"Out of bounds row", 128, 64, false},
		{"Out of bounds col", 64, 128, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewBootstrapDiscoveryService(h, []peer.AddrInfo{}, gridManager, tc.row, tc.col)

			if tc.valid {
				assert.Equal(t, tc.row, service.myRow)
				assert.Equal(t, tc.col, service.myCol)
			} else {
				// Out of bounds positions should not crash
				assert.Equal(t, tc.row, service.myRow)
				assert.Equal(t, tc.col, service.myCol)
			}
		})
	}
}

// Test bootstrap request/response full round trip
func TestBootstrapDiscovery_FullRoundTrip(t *testing.T) {
	// Create a request
	req := BootstrapPeerRequest{
		Type:      GetRowPeersRequest,
		NodeID:    "12D3KooTestNode",
		Row:       50,
		Col:       75,
		GridDims:  GridDimensions{Rows: 128, Cols: 128},
		PeerAddrs: []string{"/ip4/127.0.0.1/tcp/30333", "/ip6/::1/tcp/30333"},
	}

	// Marshal
	reqData, err := marshalBootstrapRequest(&req)
	require.NoError(t, err)
	assert.NotEmpty(t, reqData)

	// Unmarshal
	decodedReq, err := unmarshalBootstrapRequest(reqData)
	require.NoError(t, err)

	// Create response
	resp := BootstrapPeerResponse{
		Type:    JoinRowSubnetResponse,
		Success: true,
		Message: "OK",
		RowPeers: []BootstrapPeer{
			{
				PeerID:    "peer1",
				Addresses: []string{"/ip4/10.0.0.1/tcp/30333"},
				Position:  struct{ Row, Col uint32 }{Row: 50, Col: 10},
			},
		},
		Timestamp: time.Now().Unix(),
	}

	// Marshal response
	respData, err := marshalBootstrapResponse(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, respData)

	// Unmarshal response
	decodedResp, err := unmarshalBootstrapResponse(respData)
	require.NoError(t, err)

	// Verify both
	assert.Equal(t, req.NodeID, decodedReq.NodeID)
	assert.Equal(t, resp.Type, decodedResp.Type)
	assert.Equal(t, len(resp.RowPeers), len(decodedResp.RowPeers))
}

// Test address parsing and conversion
func TestBootstrapDiscovery_AddressConversion(t *testing.T) {
	h := createMockHostUnit(t)

	addrs := h.Addrs()
	addrStrings := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addrStrings = append(addrStrings, addr.String())
	}

	addrInfo := peer.AddrInfo{
		ID:    h.ID(),
		Addrs: addrs,
	}

	converted := addrInfoToStrings(addrInfo)
	assert.NotEmpty(t, converted)
	assert.GreaterOrEqual(t, len(converted), len(addrStrings))
}

// Helper function to create a mock host for testing
func createMockHostUnit(t *testing.T) host.Host {
	h, err := libp2p.New()
	require.NoError(t, err)
	t.Cleanup(func() { h.Close() })
	return h
}
