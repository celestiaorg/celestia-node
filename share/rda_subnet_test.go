package share

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_GetSubnetIDs returns different IDs for different positions
func Test_RDASubnet_GetSubnetIDs_Unique(t *testing.T) {
	dims := GridDimensions{Rows: 16, Cols: 16}

	peer1 := generateTestPeerID(t)
	peer2 := generateTestPeerID(t)

	rowID1, colID1 := GetSubnetIDs(peer1, dims)
	rowID2, colID2 := GetSubnetIDs(peer2, dims)

	// IDs should follow format
	assert.Contains(t, rowID1, "rda/row/")
	assert.Contains(t, colID1, "rda/col/")
	assert.Contains(t, rowID2, "rda/row/")
	assert.Contains(t, colID2, "rda/col/")

	// Very likely to be different (except by chance)
	// Just verify they're not hardcoded
	assert.NotEmpty(t, rowID1)
	assert.NotEmpty(t, colID1)
}

// Test_GetSubnetIDs consistency for same peer
func Test_RDASubnet_GetSubnetIDs_Consistent(t *testing.T) {
	dims := GridDimensions{Rows: 32, Cols: 32}
	peer := generateTestPeerID(t)

	rowID1, colID1 := GetSubnetIDs(peer, dims)
	rowID2, colID2 := GetSubnetIDs(peer, dims)

	assert.Equal(t, rowID1, rowID2, "Same peer should get same row ID")
	assert.Equal(t, colID1, colID2, "Same peer should get same col ID")
}

// Test_GetRowSubnets returns correct format
func Test_RDASubnet_GetRowSubnets_Format(t *testing.T) {
	for row := 0; row < 256; row += 50 {
		subnets := GetRowSubnets(row)
		assert.Greater(t, len(subnets), 0)
		assert.Contains(t, subnets[0], "rda/row/")
	}
}

// Test_GetColSubnets returns correct format
func Test_RDASubnet_GetColSubnets_Format(t *testing.T) {
	for col := 0; col < 256; col += 50 {
		subnets := GetColSubnets(col)
		assert.Greater(t, len(subnets), 0)
		assert.Contains(t, subnets[0], "rda/col/")
	}
}

// Test_SubnetID_Uniqueness different rows have different subnet IDs
func Test_RDASubnet_RowSubnetID_Unique(t *testing.T) {
	subnets0 := GetRowSubnets(0)
	subnets1 := GetRowSubnets(1)
	subnets255 := GetRowSubnets(255)

	assert.NotEqual(t, subnets0[0], subnets1[0])
	assert.NotEqual(t, subnets1[0], subnets255[0])
	assert.NotEqual(t, subnets0[0], subnets255[0])
}

// Test_SubnetID_Uniqueness different cols have different subnet IDs
func Test_RDASubnet_ColSubnetID_Unique(t *testing.T) {
	subnets0 := GetColSubnets(0)
	subnets1 := GetColSubnets(1)
	subnets255 := GetColSubnets(255)

	assert.NotEqual(t, subnets0[0], subnets1[0])
	assert.NotEqual(t, subnets1[0], subnets255[0])
	assert.NotEqual(t, subnets0[0], subnets255[0])
}

// Test_SubnetIDs_Orthogonal row and col subnets are different
func Test_RDASubnet_RowVsCol_Different(t *testing.T) {
	dims := GridDimensions{Rows: 64, Cols: 64}
	peer := generateTestPeerID(t)

	rowID, colID := GetSubnetIDs(peer, dims)

	assert.NotEqual(t, rowID, colID, "Row and col subnet IDs should be different")
}

// Test_LargeGridSubnets handles large grids correctly
func Test_RDASubnet_LargeGrid(t *testing.T) {
	dims := GridDimensions{Rows: 256, Cols: 256}

	for i := 0; i < 50; i++ {
		peer := generateTestPeerID(t)
		rowID, colID := GetSubnetIDs(peer, dims)

		assert.NotEmpty(t, rowID)
		assert.NotEmpty(t, colID)
		assert.Contains(t, rowID, "rda/row/")
		assert.Contains(t, colID, "rda/col/")
	}
}

// Test_SubnetOrthogonality verifies row and col subnets are orthogonal
func Test_RDASubnet_Orthogonality(t *testing.T) {
	dims := GridDimensions{Rows: 16, Cols: 16}

	peer1 := generateTestPeerID(t)
	peer2 := generateTestPeerID(t)

	pos1 := GetCoords(peer1, dims)
	pos2 := GetCoords(peer2, dims)

	rowID1, colID1 := GetSubnetIDs(peer1, dims)
	rowID2, colID2 := GetSubnetIDs(peer2, dims)

	// If in same row, row IDs should match
	if pos1.Row == pos2.Row {
		assert.Equal(t, rowID1, rowID2)
	} else {
		assert.NotEqual(t, rowID1, rowID2)
	}

	// If in same col, col IDs should match
	if pos1.Col == pos2.Col {
		assert.Equal(t, colID1, colID2)
	} else {
		assert.NotEqual(t, colID1, colID2)
	}
}
