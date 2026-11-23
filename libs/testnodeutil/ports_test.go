package testnodeutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFreePortsAreDistinct(t *testing.T) {
	ports := FreePorts(64)
	seen := make(map[int]struct{}, len(ports))
	for _, p := range ports {
		require.NotContains(t, seen, p)
		seen[p] = struct{}{}
	}
}
