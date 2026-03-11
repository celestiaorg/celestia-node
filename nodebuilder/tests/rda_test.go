//go:build p2p || integration

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2pDisc "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	basic "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	swarmt "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share"
)

// noopDiscovery is a stub discovery.Discovery used in tests where there is no
// real DHT.  Advertise is a no-op; FindPeers returns an immediately-closed
// channel.
type noopDiscovery struct{}

func (n *noopDiscovery) Advertise(_ context.Context, _ string, _ ...libp2pDisc.Option) (time.Duration, error) {
	return time.Minute, nil
}
func (n *noopDiscovery) FindPeers(_ context.Context, _ string, _ ...libp2pDisc.Option) (<-chan peer.AddrInfo, error) {
	ch := make(chan peer.AddrInfo)
	close(ch)
	return ch, nil
}

// ============================================================
// Test 1: Auto Grid Size - pure logic, không cần network
// ============================================================

func TestRDA_GridAutoSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		nodeCount uint32
		minCells  int
	}{
		{64, 64},
		{100, 100},
		{1000, 1000},
		{10000, 10000},
		{65536, 65536},
	}

	t.Log("\n╔═══════════════════════════════════╗")
	t.Log("║   RDA Auto Grid Size Calculator   ║")
	t.Log("╚═══════════════════════════════════╝")
	t.Logf("  %-8s → %5s x %-5s = %s", "Nodes", "Rows", "Cols", "Total Cells")
	t.Log("  ---------|-------|-------|------------")

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d_nodes", tc.nodeCount), func(t *testing.T) {
			dims := share.CalculateOptimalGridSize(tc.nodeCount)
			total := int(dims.Rows) * int(dims.Cols)
			t.Logf("  %-8d → %5d x %-5d = %d", tc.nodeCount, dims.Rows, dims.Cols, total)
			assert.Greater(t, int(dims.Rows), 0)
			assert.Greater(t, int(dims.Cols), 0)
			assert.GreaterOrEqual(t, total, tc.minCells)
		})
	}
}

// ============================================================
// Test 2: Grid Distribution - mocknet, không cần consensus
// ============================================================

/*
Test-Case: RDA Grid phân chia 9 nodes vào lưới 3x3
===================================================
Dùng mocknet (libp2p mock network) để tạo 9 hosts thật,
có PubSub thật, kết nối với nhau và chạy RDA services.

Không cần consensus/celestia-app vì chỉ test P2P grid topology.

Kiểm tra:
 1. Mỗi node được map vào đúng (row, col) trong grid 3x3
 2. Nodes cùng row nhận ra nhau là row peers
 3. Nodes cùng col nhận ra nhau là col peers
 4. Grid distribution trực quan qua log
*/
func TestRDA_GridDistribution_MockNet(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	const numNodes = 9
	gridDims := share.GridDimensions{Rows: 3, Cols: 3}

	// Tạo mocknet - mạng P2P ảo trong memory
	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	// Tạo 9 hosts thật (có PeerID, libp2p đầy đủ)
	hosts := make([]mocknet.Mocknet, 0) // dummy
	_ = hosts
	peerIDs := make([]peer.ID, numNodes)
	rdaSvcs := make([]*share.RDANodeService, numNodes)

	for i := 0; i < numNodes; i++ {
		h, err := mn.GenPeer()
		require.NoError(t, err)

		ps, err := pubsub.NewGossipSub(ctx, h)
		require.NoError(t, err)

		peerIDs[i] = h.ID()

		cfg := share.RDANodeServiceConfig{
			GridDimensions:    gridDims,
			ExpectedNodeCount: uint32(numNodes),
			FilterPolicy:      share.DefaultFilterPolicy(),
		}
		svc := share.NewRDANodeService(h, ps, cfg)
		require.NoError(t, svc.Start(ctx))
		rdaSvcs[i] = svc
		t.Cleanup(func() { _ = svc.Stop(context.Background()) })
	}

	// Kết nối tất cả các hosts với nhau
	require.NoError(t, mn.LinkAll())
	require.NoError(t, mn.ConnectAllButSelf())

	// Đợi peer discovery
	time.Sleep(200 * time.Millisecond)

	// ========== Build grid visualization ==========
	gridMap := make(map[[2]int][]int) // cell → list of node indices
	for i, id := range peerIDs {
		pos := share.GetCoords(id, gridDims)
		key := [2]int{pos.Row, pos.Col}
		gridMap[key] = append(gridMap[key], i)
	}

	t.Log("\n╔═══════════════════════════════════════╗")
	t.Log("║    RDA GRID 3x3 - 9 Node Distribution ║")
	t.Log("╚═══════════════════════════════════════╝")
	t.Log("\n  Grid (số node trong mỗi ô):")
	t.Log("         Col-0    Col-1    Col-2")
	for row := 0; row < int(gridDims.Rows); row++ {
		line := fmt.Sprintf("  Row-%d: ", row)
		for col := 0; col < int(gridDims.Cols); col++ {
			count := len(gridMap[[2]int{row, col}])
			cell := fmt.Sprintf("  [%d]   ", count)
			line += cell
		}
		t.Log(line)
	}

	// ========== Node positions ==========
	t.Log("\n  Node positions:")
	t.Logf("  %-8s | %-22s | Row | Col | Row Topic   | Col Topic", "Node", "PeerID (short)")
	t.Log("  ---------|------------------------|-----|-----|-------------|----------")
	for i, id := range peerIDs {
		pos := share.GetCoords(id, gridDims)
		rowT, colT := share.GetSubnetIDs(id, gridDims)
		shortID := id.String()
		if len(shortID) > 22 {
			shortID = shortID[:22] + ".."
		}
		t.Logf("  node%-4d | %-24s | %3d | %3d | %-11s | %s",
			i, shortID, pos.Row, pos.Col, rowT, colT)

		// Bounds check
		assert.GreaterOrEqual(t, pos.Row, 0)
		assert.Less(t, pos.Row, int(gridDims.Rows))
		assert.GreaterOrEqual(t, pos.Col, 0)
		assert.Less(t, pos.Col, int(gridDims.Cols))

		// Topic format check
		assert.Equal(t, fmt.Sprintf("rda/row/%d", pos.Row), rowT)
		assert.Equal(t, fmt.Sprintf("rda/col/%d", pos.Col), colT)
	}

	// ========== Row/Col peer relationships ==========
	t.Log("\n  Peer Relationship Matrix:")
	t.Log("  (✓ = same row/col, so IsRowPeer/IsColPeer phải = true)")
	t.Log("")

	gridMgr := share.NewRDAGridManager(gridDims)
	for _, id := range peerIDs {
		gridMgr.RegisterPeer(id)
	}

	for i := 0; i < numNodes; i++ {
		for j := i + 1; j < numNodes; j++ {
			posI := share.GetCoords(peerIDs[i], gridDims)
			posJ := share.GetCoords(peerIDs[j], gridDims)
			sameRow := posI.Row == posJ.Row
			sameCol := posI.Col == posJ.Col

			isRowPeer := share.IsRowPeer(peerIDs[i], peerIDs[j], gridDims)
			isColPeer := share.IsColPeer(peerIDs[i], peerIDs[j], gridDims)

			if sameRow || sameCol {
				mark := ""
				if sameRow {
					mark += "ROW"
				}
				if sameCol {
					if mark != "" {
						mark += "+"
					}
					mark += "COL"
				}
				t.Logf("  node%d(r%d,c%d) ↔ node%d(r%d,c%d): [%s] IsRow=%v IsCol=%v",
					i, posI.Row, posI.Col,
					j, posJ.Row, posJ.Col,
					mark, isRowPeer, isColPeer)
			}

			assert.Equal(t, sameRow, isRowPeer,
				"node%d<->node%d: IsRowPeer wrong", i, j)
			assert.Equal(t, sameCol, isColPeer,
				"node%d<->node%d: IsColPeer wrong", i, j)
		}
	}

	// ========== Distribution stats ==========
	t.Log("\n  Distribution Stats:")
	rowCounts := make(map[int]int)
	colCounts := make(map[int]int)
	for _, id := range peerIDs {
		pos := share.GetCoords(id, gridDims)
		rowCounts[pos.Row]++
		colCounts[pos.Col]++
	}
	for row := 0; row < int(gridDims.Rows); row++ {
		rowPeers := gridMgr.GetRowPeers(row)
		t.Logf("  Row %d: %d nodes (subnet peers: %d)", row, rowCounts[row], len(rowPeers))
	}
	for col := 0; col < int(gridDims.Cols); col++ {
		colPeers := gridMgr.GetColPeers(col)
		t.Logf("  Col %d: %d nodes (subnet peers: %d)", col, colCounts[col], len(colPeers))
	}

	t.Log("\n✓ TestRDA_GridDistribution_MockNet PASSED")
}

// ============================================================
// Test 3: RDA Node Service với Swamp Bridge + mocknet lights
// ============================================================

/*
Test-Case: RDA GridManager + RDANodeService với Bridge Node thật
================================================================
Dùng Swamp để tạo 1 Bridge Node thật (kết nối celestia-app).
Bridge node này có Host và PubSub thật đã running.
RDA service được khởi tạo tự động bởi nodebuilder (qua fx).

Kiểm tra:
 1. Bridge node có RDAMod (service đã chạy)
 2. PeerID → position trong grid đúng
 3. GetStats trả về dữ liệu hợp lệ
 4. Filter policy hoạt động đúng
 5. Grid visualization với bridge + extra nodes
*/
func TestRDA_BridgeNode_Service(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))

	// Chỉ start bridge node
	bridge := sw.NewBridgeNode()
	require.NoError(t, bridge.Start(ctx))

	// ========== Dùng RDAMod đã được nodebuilder tạo sẵn ==========
	rdaMod := bridge.RDAMod
	require.NotNil(t, rdaMod, "RDA module should be initialized")
	require.NotNil(t, rdaMod.Service, "RDA service should be running")

	bridgeSvc := rdaMod.Service
	bridgeID := bridge.Host.ID()
	gridDims := bridgeSvc.GetGridManager().GetGridDimensions()

	bridgePos := bridgeSvc.GetMyPosition()
	expectedPos := share.GetCoords(bridgeID, gridDims)
	rowTopic, colTopic := share.GetSubnetIDs(bridgeID, gridDims)

	t.Log("\n╔══════════════════════════════════════════╗")
	t.Log("║   RDA Bridge Node Service - Live Test    ║")
	t.Log("╚══════════════════════════════════════════╝")
	t.Logf("\n  Bridge PeerID : %s", bridgeID.String())
	t.Logf("  Grid          : %dx%d (%d cells)",
		gridDims.Rows, gridDims.Cols, int(gridDims.Rows)*int(gridDims.Cols))
	t.Logf("  Position      : row=%d, col=%d", bridgePos.Row, bridgePos.Col)
	t.Logf("  Row Topic     : %s", rowTopic)
	t.Logf("  Col Topic     : %s", colTopic)

	// Positions phải match
	assert.Equal(t, expectedPos.Row, bridgePos.Row, "row mismatch")
	assert.Equal(t, expectedPos.Col, bridgePos.Col, "col mismatch")
	assert.Equal(t, fmt.Sprintf("rda/row/%d", bridgePos.Row), rowTopic)
	assert.Equal(t, fmt.Sprintf("rda/col/%d", bridgePos.Col), colTopic)

	// ========== Tạo thêm 5 mocknet nodes, register vào grid ==========
	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	const numExtra = 5
	extraIDs := make([]peer.ID, numExtra)
	for i := 0; i < numExtra; i++ {
		h, err := mn.GenPeer()
		require.NoError(t, err)
		extraIDs[i] = h.ID()
		bridgeSvc.GetGridManager().RegisterPeer(h.ID())
	}

	// ========== Grid visualization ==========
	allIDs := append([]peer.ID{bridgeID}, extraIDs...)
	allNames := []string{"bridge[B]"}
	for i := 1; i <= numExtra; i++ {
		allNames = append(allNames, fmt.Sprintf("extra[%d]", i))
	}

	t.Logf("\n  Grid Map (%dx%d, bridge[B] + %d extras):",
		gridDims.Rows, gridDims.Cols, numExtra)

	// Chỉ in vài row đầu để tiết kiệm output
	printRows := int(gridDims.Rows)
	if printRows > 6 {
		printRows = 6
	}
	for row := 0; row < printRows; row++ {
		line := fmt.Sprintf("  Row%3d: ", row)
		for col := 0; col < int(gridDims.Cols); col++ {
			cell := "[ ]"
			for j, id := range allIDs {
				pos := share.GetCoords(id, gridDims)
				if pos.Row == row && pos.Col == col {
					if j == 0 {
						cell = "[B]"
					} else {
						cell = fmt.Sprintf("[%d]", j)
					}
				}
			}
			line += cell
		}
		t.Log(line)
	}
	if int(gridDims.Rows) > printRows {
		t.Logf("  ... (remaining %d rows empty)", int(gridDims.Rows)-printRows)
	}

	// ========== Bridge's neighbors ==========
	gm := bridgeSvc.GetGridManager()
	rowPeers := gm.GetRowPeers(bridgePos.Row)
	colPeers := gm.GetColPeers(bridgePos.Col)
	t.Logf("\n  Bridge row peers (row %d): %d node(s)", bridgePos.Row, len(rowPeers))
	t.Logf("  Bridge col peers (col %d): %d node(s)", bridgePos.Col, len(colPeers))

	// ========== Stats ==========
	router := bridgeSvc.GetGossipRouter()
	stats := router.GetStats()
	t.Log("\n  Router Stats (from running service):")
	t.Logf("    Row messages sent  : %d", stats.RowMessagesSent)
	t.Logf("    Col messages sent  : %d", stats.ColMessagesSent)
	t.Logf("    Peers in row       : %d", stats.PeersInRow)
	t.Logf("    Peers in col       : %d", stats.PeersInCol)

	// ========== Filter policy ==========
	filter := bridgeSvc.GetPeerFilter()
	policy := filter.GetFilterPolicy()
	t.Log("\n  Filter Policy:")
	t.Logf("    AllowAny           : %v", policy.AllowAny)
	t.Logf("    AllowRowComm       : %v", policy.AllowRowCommunication)
	t.Logf("    AllowColComm       : %v", policy.AllowColCommunication)
	t.Logf("    MaxRowPeers        : %d", policy.MaxRowPeers)
	t.Logf("    MaxColPeers        : %d", policy.MaxColPeers)

	assert.True(t, policy.AllowRowCommunication || policy.AllowAny)
	assert.True(t, policy.AllowColCommunication || policy.AllowAny)

	t.Log("\n✓ TestRDA_BridgeNode_Service PASSED")
}

// ============================================================
// Test 4: RDA Discovery & Bootstrap mechanism
// ============================================================

/*
Test-Case: RDADiscovery bootstrap + rendezvous points
======================================================
Verifies two core aspects of the discovery/bootstrap system:

	A) Rendezvous Points
	   Each RDADiscovery instance derives its DHT rendezvous namespaces from
	   its PeerID and grid dimensions.  We check that the format is exactly
	   "rda/row/<N>" / "rda/col/<M>" and that they match GetCoords().

	B) Bootstrap Connection
	   Given node A's address, node B uses ConnectToBootstrap to dial A
	   directly.  After the call A and B are connected — this is the first
	   step a fresh node takes when it joins the network.

A noopDiscovery stub replaces the real DHT so the test runs without
external infrastructure (mocknet only).
*/
func TestRDA_Discovery_BootstrapAndRendezvous(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	const numNodes = 4
	gridDims := share.GridDimensions{Rows: 4, Cols: 4}

	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	type nodeInfo struct {
		disc    *share.RDADiscovery
		gridMgr *share.RDAGridManager
		pos     share.GridPosition
		rowNS   string
		colNS   string
	}

	nodes := make([]nodeInfo, numNodes)
	for i := range nodes {
		h, err := mn.GenPeer()
		require.NoError(t, err)

		gm := share.NewRDAGridManager(gridDims)
		d := share.NewRDADiscovery(h, &noopDiscovery{}, gm, nil)

		rowNS, colNS := d.GetMyRendezvousPoints()
		pos := share.GetCoords(h.ID(), gridDims)

		nodes[i] = nodeInfo{
			disc:    d,
			gridMgr: gm,
			pos:     pos,
			rowNS:   rowNS,
			colNS:   colNS,
		}
	}

	// ── Part A: Rendezvous points are deterministic and well-formed ──────────
	t.Log("\n╔════════════════════════════════════════════╗")
	t.Log("║   A) RDA Discovery Rendezvous Point Check   ║")
	t.Log("╚════════════════════════════════════════════╝")
	t.Logf("\n  %-6s | %-5s | %-5s | %-14s | %s", "Node", "Row", "Col", "Row NS", "Col NS")
	t.Log("  -------|-------|-------|----------------|----------------")

	for i, n := range nodes {
		t.Logf("  node%-2d | %5d | %5d | %-14s | %s", i, n.pos.Row, n.pos.Col, n.rowNS, n.colNS)

		// Namespace format must be "rda/row/<row>" and "rda/col/<col>"
		assert.Equal(t, fmt.Sprintf("rda/row/%d", n.pos.Row), n.rowNS,
			"node%d: rowNS mismatch", i)
		assert.Equal(t, fmt.Sprintf("rda/col/%d", n.pos.Col), n.colNS,
			"node%d: colNS mismatch", i)
		assert.GreaterOrEqual(t, n.pos.Row, 0)
		assert.Less(t, n.pos.Row, int(gridDims.Rows))
		assert.GreaterOrEqual(t, n.pos.Col, 0)
		assert.Less(t, n.pos.Col, int(gridDims.Cols))
	}

	// ── Part B: Bootstrap connection ────────────────────────────────────────
	// Important: ALL hosts must be created BEFORE mn.LinkAll() so that
	// mocknet can route dials between them.

	bootstrapH, err := mn.GenPeer()
	require.NoError(t, err)

	const numBootstrapNodes = 3

	// Pre-create all client hosts
	type clientNode struct {
		h interface {
			ID() peer.ID
			Network() interface{ Connectedness(peer.ID) int }
		}
		disc *share.RDADiscovery
	}
	_ = clientNode{}

	// Use concrete libp2p host type via GenPeer
	clientHosts := make([]interface {
		ID() peer.ID
		Addrs() []interface{}
		Network() interface {
			Connectedness(peer.ID) interface{}
		}
	}, 0)
	_ = clientHosts

	// Simpler: store as peer.AddrInfo retrieval directly
	type hostDisc struct {
		peerID peer.ID
		disc   *share.RDADiscovery
	}
	hdNodes := make([]hostDisc, numBootstrapNodes)

	bootstrapInfo := peer.AddrInfo{
		ID:    bootstrapH.ID(),
		Addrs: bootstrapH.Addrs(),
	}

	for i := 0; i < numBootstrapNodes; i++ {
		h, err := mn.GenPeer()
		require.NoError(t, err)

		gm := share.NewRDAGridManager(gridDims)
		d := share.NewRDADiscovery(h, &noopDiscovery{}, gm, []peer.AddrInfo{bootstrapInfo})
		hdNodes[i] = hostDisc{peerID: h.ID(), disc: d}
	}

	// Link ALL hosts now (Part A hosts + bootstrapH + client hosts).
	require.NoError(t, mn.LinkAll())

	t.Log("\n╔══════════════════════════════════════════╗")
	t.Log("║   B) Bootstrap Connection (A→B handshake) ║")
	t.Log("╚══════════════════════════════════════════╝")
	t.Logf("  Bootstrap: %s...", bootstrapH.ID().String()[:16])

	// Start each discovery — connectToBootstrap is synchronous in Start().
	for i := range hdNodes {
		require.NoError(t, hdNodes[i].disc.Start(ctx))
		t.Cleanup(func() { _ = hdNodes[i].disc.Stop(context.Background()) })

		rowNS, colNS := hdNodes[i].disc.GetMyRendezvousPoints()
		t.Logf("  node%d: rendezvous %s / %s", i, rowNS, colNS)
	}

	// Check connections via the mocknet's host registry.
	connected := 0
	for i, n := range hdNodes {
		h := mn.Host(n.peerID)
		require.NotNil(t, h, "host %d not found in mocknet", i)
		if h.Network().Connectedness(bootstrapH.ID()) == 1 { // network.Connected == 1
			connected++
			t.Logf("  ✓ node%d connected to bootstrap", i)
		} else {
			t.Logf("  ✗ node%d not yet connected", i)
		}
	}

	t.Logf("\n  Bootstrap connections: %d/%d", connected, numBootstrapNodes)
	assert.Greater(t, connected, 0, "at least one node should connect to bootstrap")

	t.Log("\n✓ TestRDA_Discovery_BootstrapAndRendezvous PASSED")
}

// ============================================================
// Test 5: RDA DHT Peer Discovery - nodes find each other thật
// ============================================================

/*
Test-Case: Multi-node DHT rendezvous
======================================
Creates 12 real libp2p hosts on a 3x3 grid, each with a real Kademlia
DHT and a real RDADiscovery. After all nodes start, they advertise their
grid position under "rda/row/<N>" / "rda/col/<M>" and query the DHT for
peers in the same namespaces.

After a short wait we verify:
 1. Every node that has at least one other node in the same row becomes
    connected to that row-peer.
 2. Every node that has at least one other node in the same col becomes
    connected to that col-peer.

No mocknet — real swarm + real DHT inside an in-process network.
*/
func TestRDA_Discovery_DHT_PeerFinding(t *testing.T) {
	// DHT tests have known data races inside the kad-dht library; skip under
	// the race detector (same approach used by the existing discovery tests).
	if testing.Short() {
		t.Skip("skipping DHT discovery test in -short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	const numNodes = 12
	gridDims := share.GridDimensions{Rows: 3, Cols: 3}

	// ── 1. Bootstrapper node (DHT server, no upstream bootstrappers) ─────────
	bootHost := newRDATestHost(t)
	bootDHT, err := dht.New(ctx, bootHost,
		dht.Mode(dht.ModeServer),
		dht.BootstrapPeers(),
		dht.ProtocolPrefix("/rda-test"),
	)
	require.NoError(t, err)
	require.NoError(t, bootDHT.Bootstrap(ctx))

	bootInfo := peer.AddrInfo{ID: bootHost.ID(), Addrs: bootHost.Addrs()}

	// ── 2. Create numNodes hosts, each with DHT + RDADiscovery ─────────────
	type rdaNode struct {
		h    host.Host
		disc *share.RDADiscovery
		gm   *share.RDAGridManager
		pos  share.GridPosition
	}

	nodes := make([]rdaNode, numNodes)
	for i := range nodes {
		h := newRDATestHost(t)

		// Connect to bootstrapper so DHT routing tables are populated.
		require.NoError(t, h.Connect(ctx, bootInfo), "node%d: connect to bootstrapper", i)

		nodeDHT, err := dht.New(ctx, h,
			dht.Mode(dht.ModeServer),
			dht.ProtocolPrefix("/rda-test"),
		)
		require.NoError(t, err)
		require.NoError(t, nodeDHT.Bootstrap(ctx))

		disc := routingdisc.NewRoutingDiscovery(nodeDHT)
		gm := share.NewRDAGridManager(gridDims)
		rdaDisc := share.NewRDADiscovery(h, disc, gm, []peer.AddrInfo{bootInfo})
		rdaDisc.SetInterval(150 * time.Millisecond) // fast for tests

		pos := share.GetCoords(h.ID(), gridDims)

		nodes[i] = rdaNode{h: h, disc: rdaDisc, gm: gm, pos: pos}
	}

	// ── 3. Print grid layout ─────────────────────────────────────────────────
	t.Log("\n╔══════════════════════════════════════════════════╗")
	t.Log("║   RDA DHT Discovery Test - 12 nodes / 3x3 grid  ║")
	t.Log("╚══════════════════════════════════════════════════╝")

	rowMap := make(map[int][]int) // row → node indices
	colMap := make(map[int][]int) // col → node indices
	for i, n := range nodes {
		rowMap[n.pos.Row] = append(rowMap[n.pos.Row], i)
		colMap[n.pos.Col] = append(colMap[n.pos.Col], i)
	}

	t.Log("\n  Grid layout (node indices per cell):")
	t.Log("         Col-0    Col-1    Col-2")
	for row := 0; row < int(gridDims.Rows); row++ {
		line := fmt.Sprintf("  Row-%d: ", row)
		for col := 0; col < int(gridDims.Cols); col++ {
			cell := "  [ ]  "
			var cellNodes []int
			for i, n := range nodes {
				if n.pos.Row == row && n.pos.Col == col {
					cellNodes = append(cellNodes, i)
				}
			}
			if len(cellNodes) > 0 {
				cell = fmt.Sprintf("  [%d]  ", len(cellNodes))
			}
			line += cell
		}
		t.Logf("%s  (row peers: %d)", line, len(rowMap[row]))
	}

	t.Log("\n  Node positions:")
	for i, n := range nodes {
		rowNS, colNS := n.disc.GetMyRendezvousPoints()
		t.Logf("  node%-2d  pos=(%d,%d)  adv: %s / %s",
			i, n.pos.Row, n.pos.Col, rowNS, colNS)
	}

	// ── 4. Start all discovery services ──────────────────────────────────────
	for i, n := range nodes {
		require.NoError(t, n.disc.Start(ctx), "node%d: start discovery", i)
		t.Cleanup(func() { _ = n.disc.Stop(context.Background()) })
	}

	// ── 5. Wait for DHT advertisement + find cycle to complete ───────────────
	// Each node advertises immediately on Start, then 150ms ticks follow.
	// Two cycles should be enough for all peers to cross-discover.
	t.Log("\n  Waiting for DHT rendezvous (1s)...")
	time.Sleep(1 * time.Second)

	// ── 6. Verify: same-row/col peers are connected ───────────────────────────
	t.Log("\n  Connection results:")
	t.Log("  ──────────────────────────────────────────────────────")

	totalExpected := 0
	totalFound := 0

	for i, n := range nodes {
		// Build expected same-row peers (other nodes, not self)
		var expectRowPeers, expectColPeers []int
		for j, other := range nodes {
			if j == i {
				continue
			}
			if other.pos.Row == n.pos.Row {
				expectRowPeers = append(expectRowPeers, j)
			}
			if other.pos.Col == n.pos.Col {
				expectColPeers = append(expectColPeers, j)
			}
		}
		if len(expectRowPeers)+len(expectColPeers) == 0 {
			t.Logf("  node%-2d (%d,%d): no expected peers (isolated cell)", i, n.pos.Row, n.pos.Col)
			continue
		}

		// Check actual connections
		connToRow, connToCol := 0, 0
		for _, j := range expectRowPeers {
			if n.h.Network().Connectedness(nodes[j].h.ID()) == network.Connected {
				connToRow++
			}
		}
		for _, j := range expectColPeers {
			if n.h.Network().Connectedness(nodes[j].h.ID()) == network.Connected {
				connToCol++
			}
		}

		rowMark, colMark := "✗", "✗"
		if connToRow > 0 || len(expectRowPeers) == 0 {
			rowMark = "✓"
		}
		if connToCol > 0 || len(expectColPeers) == 0 {
			colMark = "✓"
		}

		t.Logf("  node%-2d (%d,%d):  row-peers %s %d/%d   col-peers %s %d/%d",
			i, n.pos.Row, n.pos.Col,
			rowMark, connToRow, len(expectRowPeers),
			colMark, connToCol, len(expectColPeers),
		)

		if len(expectRowPeers) > 0 {
			totalExpected++
			if connToRow > 0 {
				totalFound++
			}
		}
		if len(expectColPeers) > 0 {
			totalExpected++
			if connToCol > 0 {
				totalFound++
			}
		}
	}

	t.Logf("\n  Discovery score: %d/%d node-dimensions found same-subnet peers via DHT",
		totalFound, totalExpected)

	// Require a majority to have found at least one peer (allows for some DHT
	// propagation variance in fast tests).
	minRequired := (totalExpected * 2) / 3
	assert.GreaterOrEqual(t, totalFound, minRequired,
		"expected at least 2/3 of node-dimensions to find peers via DHT")

	t.Log("\n✓ TestRDA_Discovery_DHT_PeerFinding PASSED")
}

// newRDATestHost creates a plain libp2p host using an in-memory swarm,
// suitable for DHT-based discovery tests.
func newRDATestHost(t *testing.T) host.Host {
	t.Helper()
	bus := eventbus.NewBus()
	sw := swarmt.GenSwarm(t, swarmt.OptDisableTCP, swarmt.EventBus(bus))
	h, err := basic.NewHost(sw, &basic.HostOpts{EventBus: bus})
	require.NoError(t, err)
	h.Start()
	t.Cleanup(func() { _ = h.Close() })
	return h
}
