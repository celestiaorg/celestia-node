package node

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node/node"
)

func TestLifecycle(t *testing.T) {
	var test = []struct {
		tp           node.Type
		coreExpected bool
	}{
		{tp: node.Bridge, coreExpected: true},
		{tp: node.Full},
		{tp: node.Light},
	}

	for i, tt := range test {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			node := TestNode(t, tt.tp)
			require.NotNil(t, node)
			require.NotNil(t, node.Config)
			require.NotNil(t, node.Host)
			require.NotNil(t, node.HeaderServ)
			require.NotNil(t, node.StateServ)
			require.Equal(t, tt.tp, node.Type)

			if tt.coreExpected {
				require.NotNil(t, node.CoreClient)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := node.Start(ctx)
			require.NoError(t, err)

			err = node.Stop(ctx)
			require.NoError(t, err)
		})
	}
}
