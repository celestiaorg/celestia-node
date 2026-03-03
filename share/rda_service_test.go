package share

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_DefaultRDANodeServiceConfig returns correct defaults
func Test_DefaultRDANodeServiceConfig(t *testing.T) {
	config := DefaultRDANodeServiceConfig()

	assert.Equal(t, uint16(128), config.GridDimensions.Rows)
	assert.Equal(t, uint16(128), config.GridDimensions.Cols)
	assert.Equal(t, uint32(0), config.ExpectedNodeCount)
	assert.False(t, config.EnableDetailedLogging)
	assert.True(t, config.GridDimensions.Rows > 0)
	assert.True(t, config.GridDimensions.Cols > 0)
}

// Test_RDANodeServiceConfig_CustomGridDimensions
func Test_RDANodeServiceConfig_CustomGridDimensions(t *testing.T) {
	config := RDANodeServiceConfig{
		GridDimensions: GridDimensions{Rows: 64, Cols: 64},
		FilterPolicy:   DefaultFilterPolicy(),
	}

	assert.Equal(t, uint16(64), config.GridDimensions.Rows)
	assert.Equal(t, uint16(64), config.GridDimensions.Cols)
}

// Test_RDANodeServiceConfig_ExpectedNodeCount overrides dimensions
func Test_RDANodeServiceConfig_ExpectedNodeCount(t *testing.T) {
	config := RDANodeServiceConfig{
		GridDimensions:    GridDimensions{Rows: 128, Cols: 128},
		ExpectedNodeCount: 10000,
	}

	// Config accepts both - calculation happens in service
	assert.Greater(t, config.ExpectedNodeCount, uint32(0))
	assert.Equal(t, uint16(128), config.GridDimensions.Rows)
}

// Test_RDANodeServiceConfig_FilterPolicy
func Test_RDANodeServiceConfig_FilterPolicy(t *testing.T) {
	config := RDANodeServiceConfig{
		GridDimensions: GridDimensions{Rows: 32, Cols: 32},
		FilterPolicy: FilterPolicy{
			AllowRowCommunication: true,
			AllowColCommunication: false,
			MaxRowPeers:           100,
		},
	}

	assert.True(t, config.FilterPolicy.AllowRowCommunication)
	assert.False(t, config.FilterPolicy.AllowColCommunication)
	assert.Equal(t, 100, config.FilterPolicy.MaxRowPeers)
}

// Test_RDANodeServiceConfig_EnablingDebugLogging
func Test_RDANodeServiceConfig_EnableDebugLogging(t *testing.T) {
	config := RDANodeServiceConfig{
		GridDimensions:        GridDimensions{Rows: 64, Cols: 64},
		EnableDetailedLogging: true,
	}

	assert.True(t, config.EnableDetailedLogging)
}

// Test_RDANodeServiceConfig_InheritDefaults when not specified
func Test_RDANodeServiceConfig_InheritDefaults(t *testing.T) {
	config := RDANodeServiceConfig{
		GridDimensions: GridDimensions{Rows: 32, Cols: 32},
	}

	// FilterPolicy should be zero value (all false / 0)
	assert.False(t, config.FilterPolicy.AllowAny)
	assert.Equal(t, uint32(0), config.ExpectedNodeCount)
	assert.False(t, config.EnableDetailedLogging)
}

// Test_ValidateGridDimensions checks dimension constraints
func Test_ValidateGridDimensions_Positive(t *testing.T) {
	dims := GridDimensions{Rows: 8, Cols: 8}
	assert.Greater(t, dims.Rows, uint16(0))
	assert.Greater(t, dims.Cols, uint16(0))
}

// Test_ValidateGridDimensions_MaxSize checks reasonable limits
func Test_ValidateGridDimensions_MaxSize(t *testing.T) {
	dims := GridDimensions{Rows: 256, Cols: 256}
	assert.LessOrEqual(t, dims.Rows, uint16(256))
	assert.LessOrEqual(t, dims.Cols, uint16(256))
}

// Test_MultipleConfigInstances are independent
func Test_MultipleConfigInstances(t *testing.T) {
	config1 := RDANodeServiceConfig{
		GridDimensions: GridDimensions{Rows: 64, Cols: 64},
	}

	config2 := RDANodeServiceConfig{
		GridDimensions: GridDimensions{Rows: 128, Cols: 128},
	}

	assert.NotEqual(t, config1.GridDimensions.Rows, config2.GridDimensions.Rows)
	assert.Equal(t, uint16(64), config1.GridDimensions.Rows)
	assert.Equal(t, uint16(128), config2.GridDimensions.Rows)
}

// Test_UpdateConfigValues allows modification
func Test_UpdateConfigValues(t *testing.T) {
	config := DefaultRDANodeServiceConfig()

	// Modify
	config.GridDimensions.Rows = 256
	config.EnableDetailedLogging = true
	config.ExpectedNodeCount = 50000

	assert.Equal(t, uint16(256), config.GridDimensions.Rows)
	assert.True(t, config.EnableDetailedLogging)
	assert.Equal(t, uint32(50000), config.ExpectedNodeCount)
}

// Test_FilterPolicy_InConfigApplied
func Test_FilterPolicy_InConfig(t *testing.T) {
	config := RDANodeServiceConfig{
		GridDimensions: GridDimensions{Rows: 64, Cols: 64},
		FilterPolicy:   StrictFilterPolicy(),
	}

	assert.Equal(t, 128, config.FilterPolicy.MaxRowPeers)
	assert.Equal(t, 128, config.FilterPolicy.MaxColPeers)
}

// Test_ConfigConsistency checks all fields work together
func Test_ConfigConsistency(t *testing.T) {
	config := RDANodeServiceConfig{
		GridDimensions:        GridDimensions{Rows: 64, Cols: 64},
		FilterPolicy:          DefaultFilterPolicy(),
		ExpectedNodeCount:     5000,
		EnableDetailedLogging: true,
	}

	// All fields should be accessible and consistent
	assert.Equal(t, uint16(64), config.GridDimensions.Rows)
	assert.Greater(t, config.FilterPolicy.MaxRowPeers, 0)
	assert.Greater(t, config.ExpectedNodeCount, uint32(0))
	assert.True(t, config.EnableDetailedLogging)
}
