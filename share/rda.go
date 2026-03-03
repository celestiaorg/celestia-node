package share

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// GridDimensions định nghĩa kích thước ma trận mạng lưới RDA
// Giá trị mặc định cho mạng ~16,000 nodes: 128x128
type GridDimensions struct {
	Rows uint16
	Cols uint16
}

// DefaultGridDimensions cấu hình mặc định
var DefaultGridDimensions = GridDimensions{
	Rows: 128, // Số lượng Hàng (Rows)
	Cols: 128, // Số lượng Cột (Columns)
}

// RDAGridManager quản lý toàn bộ grid topology
type RDAGridManager struct {
	dims     GridDimensions
	peerGrid map[string]GridPosition // peerID -> position
	mu       sync.RWMutex
}

// GridPosition biểu diễn vị trí của node trong grid
type GridPosition struct {
	Row int
	Col int
}

// NewRDAGridManager khởi tạo một RDAGridManager
func NewRDAGridManager(dims GridDimensions) *RDAGridManager {
	return &RDAGridManager{
		dims:     dims,
		peerGrid: make(map[string]GridPosition),
	}
}

// SetGridDimensions thiết lập kích thước grid mới
func (g *RDAGridManager) SetGridDimensions(dims GridDimensions) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.dims = dims
	// Clear existing grid when dimensions change
	g.peerGrid = make(map[string]GridPosition)
}

// GetCoords chuyển đổi PeerID của libp2p thành tọa độ (row, col)
// Sử dụng hash để đảm bảo deterministic placement
func GetCoords(id peer.ID, dims GridDimensions) GridPosition {
	// Băm PeerID để đảm bảo tính ngẫu nhiên nhưng cố định
	hash := sha256.Sum256([]byte(id))

	// Lấy 8 byte đầu tiên để chuyển thành số nguyên
	val := binary.BigEndian.Uint64(hash[:8])

	// Tính toán hàng và cột dựa trên Modulo
	row := int(val % uint64(dims.Rows))
	col := int((val / uint64(dims.Rows)) % uint64(dims.Cols))

	return GridPosition{Row: row, Col: col}
}

// GetSubnetIDs trả về định danh chuỗi cho Subnet của hàng và cột
// Dùng để đăng ký các Topic trong GossipSub hoặc Discovery
func GetSubnetIDs(id peer.ID, dims GridDimensions) (rowID string, colID string) {
	pos := GetCoords(id, dims)
	return fmt.Sprintf("rda/row/%d", pos.Row), fmt.Sprintf("rda/col/%d", pos.Col)
}

// GetRowSubnets trả về tất cả rowID cho một row cụ thể
func GetRowSubnets(row int) []string {
	return []string{fmt.Sprintf("rda/row/%d", row)}
}

// GetColSubnets trả về tất cả colID cho một column cụ thể
func GetColSubnets(col int) []string {
	return []string{fmt.Sprintf("rda/col/%d", col)}
}

// RegisterPeer đăng ký một peer vào grid
func (g *RDAGridManager) RegisterPeer(peerID peer.ID) GridPosition {
	g.mu.Lock()
	defer g.mu.Unlock()

	pos := GetCoords(peerID, g.dims)
	g.peerGrid[peerID.String()] = pos
	return pos
}

// GetPeerPosition trả về vị trí của một peer
func (g *RDAGridManager) GetPeerPosition(peerID peer.ID) (GridPosition, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	pos, exists := g.peerGrid[peerID.String()]
	return pos, exists
}

// GetRowPeers trả về tất cả peers trong cùng hàng
func (g *RDAGridManager) GetRowPeers(row int) []peer.ID {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var peers []peer.ID
	for peerStr, pos := range g.peerGrid {
		if pos.Row == row {
			peerID, err := peer.Decode(peerStr)
			if err == nil {
				peers = append(peers, peerID)
			}
		}
	}
	return peers
}

// GetColPeers trả về tất cả peers trong cùng cột
func (g *RDAGridManager) GetColPeers(col int) []peer.ID {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var peers []peer.ID
	for peerStr, pos := range g.peerGrid {
		if pos.Col == col {
			peerID, err := peer.Decode(peerStr)
			if err == nil {
				peers = append(peers, peerID)
			}
		}
	}
	return peers
}

// IsRowPeer kiểm tra nếu hai peers trong cùng hàng
func IsRowPeer(peerID1, peerID2 peer.ID, dims GridDimensions) bool {
	pos1 := GetCoords(peerID1, dims)
	pos2 := GetCoords(peerID2, dims)
	return pos1.Row == pos2.Row
}

// IsColPeer kiểm tra nếu hai peers trong cùng cột
func IsColPeer(peerID1, peerID2 peer.ID, dims GridDimensions) bool {
	pos1 := GetCoords(peerID1, dims)
	pos2 := GetCoords(peerID2, dims)
	return pos1.Col == pos2.Col
}

// IsInSameSubnet kiểm tra nếu hai peers trong cùng row hoặc column subnet
func IsInSameSubnet(peerID1, peerID2 peer.ID, dims GridDimensions) bool {
	return IsRowPeer(peerID1, peerID2, dims) || IsColPeer(peerID1, peerID2, dims)
}

// GetGridDimensions trả về kích thước grid hiện tại
func (g *RDAGridManager) GetGridDimensions() GridDimensions {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.dims
}

// CalculateOptimalGridSize tính toán kích thước grid optimal cho số lượng nodes
func CalculateOptimalGridSize(numNodes uint32) GridDimensions {
	// Tính toán căn bậc 2 của số nodes
	sqrtNodes := uint16(math.Ceil(math.Sqrt(float64(numNodes))))

	// Đảm bảo grid không quá nhỏ
	if sqrtNodes < 16 {
		sqrtNodes = 16
	}

	// Đảm bảo grid không quá lớn
	if sqrtNodes > 256 {
		sqrtNodes = 256
	}

	return GridDimensions{
		Rows: sqrtNodes,
		Cols: sqrtNodes,
	}
}

// GetPeerSubnetTopics trả về tất cả các pubsub topics của một peer
func GetPeerSubnetTopics(peerID peer.ID, dims GridDimensions) []string {
	rowID, colID := GetSubnetIDs(peerID, dims)
	return []string{rowID, colID}
}

// FilterPeersForSubnet lọc danh sách peers để chỉ lấy những peers trong cùng subnet
func FilterPeersForSubnet(peers []peer.ID, myPeerID peer.ID, dims GridDimensions) []peer.ID {
	var filtered []peer.ID
	for _, p := range peers {
		if IsInSameSubnet(myPeerID, p, dims) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}
