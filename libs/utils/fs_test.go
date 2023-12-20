package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp(tmpDir, "test")
	require.NoError(t, err)

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
			path:     filepath.Join(tmpDir, "nonexistentfile.xyz"),
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
