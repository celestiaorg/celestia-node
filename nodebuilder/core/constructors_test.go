package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTokenPath(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		jsonContent   string
		expectedToken string
		expectError   bool
		errorContains string
	}{
		{
			name:          "x-token key with xtoken.json filename",
			filename:      "xtoken.json",
			jsonContent:   `{"x-token": "test-token-123"}`,
			expectedToken: "test-token-123",
			expectError:   false,
		},
		{
			name:          "x-token key with x-token.json filename",
			filename:      "x-token.json",
			jsonContent:   `{"x-token": "test-token-456"}`,
			expectedToken: "test-token-456",
			expectError:   false,
		},
		{
			name:          "xtoken key with xtoken.json filename",
			filename:      "xtoken.json",
			jsonContent:   `{"xtoken": "test-token-789"}`,
			expectedToken: "test-token-789",
			expectError:   false,
		},
		{
			name:          "xtoken key with x-token.json filename",
			filename:      "x-token.json",
			jsonContent:   `{"xtoken": "test-token-abc"}`,
			expectedToken: "test-token-abc",
			expectError:   false,
		},
		{
			name:          "token key with xtoken.json filename (QuickNode style)",
			filename:      "xtoken.json",
			jsonContent:   `{"token": "hunter2"}`,
			expectedToken: "hunter2",
			expectError:   false,
		},
		{
			name:          "token key with x-token.json filename",
			filename:      "x-token.json",
			jsonContent:   `{"token": "hunter3"}`,
			expectedToken: "hunter3",
			expectError:   false,
		},
		{
			name:          "x-token key takes precedence over xtoken",
			filename:      "xtoken.json",
			jsonContent:   `{"x-token": "priority-token", "xtoken": "should-be-ignored"}`,
			expectedToken: "priority-token",
			expectError:   false,
		},
		{
			name:          "x-token key takes precedence over token",
			filename:      "xtoken.json",
			jsonContent:   `{"x-token": "priority-token", "token": "should-be-ignored"}`,
			expectedToken: "priority-token",
			expectError:   false,
		},
		{
			name:          "xtoken key takes precedence over token",
			filename:      "xtoken.json",
			jsonContent:   `{"xtoken": "priority-token", "token": "should-be-ignored"}`,
			expectedToken: "priority-token",
			expectError:   false,
		},
		{
			name:          "empty token value",
			filename:      "xtoken.json",
			jsonContent:   `{"x-token": ""}`,
			expectedToken: "",
			expectError:   true,
			errorContains: "authentication token is empty",
		},
		{
			name:          "missing token key",
			filename:      "xtoken.json",
			jsonContent:   `{"other-key": "value"}`,
			expectedToken: "",
			expectError:   true,
			errorContains: "authentication token is empty or missing",
		},
		{
			name:          "invalid JSON",
			filename:      "xtoken.json",
			jsonContent:   `invalid json`,
			expectedToken: "",
			expectError:   true,
			errorContains: "failed to parse token file",
		},
		{
			name:          "file not found",
			filename:      "nonexistent.json",
			jsonContent:   ``,
			expectedToken: "",
			expectError:   true,
			errorContains: "authentication token file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := t.TempDir()

			if tt.jsonContent != "" {
				tokenFile := filepath.Join(testDir, tt.filename)
				err := os.WriteFile(tokenFile, []byte(tt.jsonContent), 0o644)
				require.NoError(t, err)
			}

			token, err := parseTokenPath(testDir)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedToken, token)
			}
		})
	}
}

func TestParseTokenPathFilenamePreference(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both files - xtoken.json should be preferred
	xtokenFile := filepath.Join(tmpDir, "xtoken.json")
	xTokenFile := filepath.Join(tmpDir, "x-token.json")

	err := os.WriteFile(xtokenFile, []byte(`{"x-token": "from-xtoken.json"}`), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(xTokenFile, []byte(`{"x-token": "from-x-token.json"}`), 0o644)
	require.NoError(t, err)

	// Should prefer xtoken.json
	token, err := parseTokenPath(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "from-xtoken.json", token)

	// Remove xtoken.json, should use x-token.json
	err = os.Remove(xtokenFile)
	require.NoError(t, err)

	token, err = parseTokenPath(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "from-x-token.json", token)
}
