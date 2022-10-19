package nodebuilder

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestLifecycle(t *testing.T) {
	var test = []struct {
		tp           node.Type
		coreExpected bool
	}{
		{tp: node.Bridge},
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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := node.Start(ctx)
			require.NoError(t, err)

			// ensure the state service is running
			require.False(t, node.StateServ.IsStopped())

			err = node.Stop(ctx)
			require.NoError(t, err)

			// ensure the state service is stopped
			require.True(t, node.StateServ.IsStopped())
		})
	}
}
