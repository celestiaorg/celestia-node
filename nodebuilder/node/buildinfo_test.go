package node

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmptyValue(t *testing.T) {
	want := "unknown"
	got := emptyValue

	require.Equal(t, want, got)
}

func TestGetSemanticVersion(t *testing.T) {
	tests := []struct {
		name            string
		buildInfo       BuildInfo
		expectedVersion string
	}{
		{
			name: "Empty Semantic Version",
			buildInfo: BuildInfo{
				SemanticVersion: "",
			},
			expectedVersion: emptyValue,
		},
		{
			name: "Non-empty Semantic Version",
			buildInfo: BuildInfo{
				SemanticVersion: "1.2.3",
			},
			expectedVersion: "v1.2.3",
		},
		// Add more test cases as needed
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualVersion := tc.buildInfo.GetSemanticVersion()
			if actualVersion != tc.expectedVersion {
				t.Errorf("Test %s failed: expected %s, got %s", tc.name, tc.expectedVersion, actualVersion)
			}
		})
	}
}

func TestBuildInfo_CommitShortSha(t *testing.T) {
	tests := []struct {
		name       string
		lastCommit string
		want       string
	}{
		{
			name:       "Empty lastCommit",
			lastCommit: "",
			want:       "unknown",
		},
		{
			name:       "Short lastCommit",
			lastCommit: "abc123",
			want:       "abc123",
		},
		{
			name:       "Valid lastCommit",
			lastCommit: "abcdefg",
			want:       "abcdefg",
		},
		{
			name:       "Long lastCommit",
			lastCommit: "abcdefghijk",
			want:       "abcdefg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildInfo{
				LastCommit: tt.lastCommit,
			}
			got := b.CommitShortSha()
			require.Equal(t, tt.want, got)
		})
	}
}
