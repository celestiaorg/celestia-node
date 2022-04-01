//go:build test_unit

package node

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_StateServiceConstruction(t *testing.T) {
	for _, tp := range []Type{Bridge, Full, Light} {
		node := TestNode(t, tp)
		// check to ensure node's state service is not nil
		require.NotNil(t, node.StateServ)
	}
}
