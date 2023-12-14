package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "File exists",
			path:     tmpFile.Name(),
			expected: true,
		},
		{
			name:     "File does not exist",
			path:     "nonexistentfile.xyz",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Exists(tc.path)
			require.Equal(t, tc.expected, result)
		})
	}
}
