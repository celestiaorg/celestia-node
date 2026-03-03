package share

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_GetCoords_Deterministic verifies that GetCoords is deterministic
func Test_GetCoords_Deterministic(t *testing.T) {
	dims := GridDimensions{Rows: 128, Cols: 128}
	testID := generateTestPeerID(t)

	pos1 := GetCoords(testID, dims)
	pos2 := GetCoords(testID, dims)

	assert.Equal(t, pos1, pos2, "GetCoords should be deterministic")
}

// Test_GetCoords_WithinBounds verifies coordinates are within grid
func Test_GetCoords_WithinBounds(t *testing.T) {
	dims := GridDimensions{Rows: 64, Cols: 64}
	testID := generateTestPeerID(t)

	pos := GetCoords(testID, dims)

	assert.True(t, pos.Row >= 0 && pos.Row < int(dims.Rows), "Row out of bounds")
	assert.True(t, pos.Col >= 0 && pos.Col < int(dims.Cols), "Col out of bounds")
}

// Test_NewRDAGridManager tests initialization
func Test_NewRDAGridManager(t *testing.T) {
	dims := GridDimensions{Rows: 32, Cols: 32}
	mgr := NewRDAGridManager(dims)

	assert.NotNil(t, mgr)
	assert.Equal(t, dims.Rows, mgr.dims.Rows)
	assert.Equal(t, dims.Cols, mgr.dims.Cols)
}

// Test_RegisterPeer_And_GetPeerPosition tests peer registration
func Test_RegisterPeer_And_GetPeerPosition(t *testing.T) {
	dims := GridDimensions{Rows: 64, Cols: 64}
	mgr := NewRDAGridManager(dims)

	testID := generateTestPeerID(t)

	pos := mgr.RegisterPeer(testID)

	assert.True(t, pos.Row >= 0 && pos.Row < int(dims.Rows))
	assert.True(t, pos.Col >= 0 && pos.Col < int(dims.Cols))

	retrievedPos, exists := mgr.GetPeerPosition(testID)
	assert.True(t, exists)
	assert.Equal(t, pos, retrievedPos)
}

// Test_SetGridDimensions tests changing grid dimensions
func Test_SetGridDimensions(t *testing.T) {
	mgr := NewRDAGridManager(GridDimensions{Rows: 64, Cols: 64})

	testID := generateTestPeerID(t)

	mgr.RegisterPeer(testID)

	newDims := GridDimensions{Rows: 128, Cols: 128}
	mgr.SetGridDimensions(newDims)

	_, exists := mgr.GetPeerPosition(testID)
	assert.False(t, exists, "Peer should be removed when dimensions change")
}

// Test_GetSubnetIDs returns correct format
func Test_GetSubnetIDs(t *testing.T) {
	dims := GridDimensions{Rows: 64, Cols: 64}
	testID := generateTestPeerID(t)

	rowID, colID := GetSubnetIDs(testID, dims)

	assert.Contains(t, rowID, "rda/row/")
	assert.Contains(t, colID, "rda/col/")
}

// Test_DefaultGridDimensions is set correctly
func Test_DefaultGridDimensions(t *testing.T) {
	assert.Equal(t, uint16(128), DefaultGridDimensions.Rows)
	assert.Equal(t, uint16(128), DefaultGridDimensions.Cols)
}
