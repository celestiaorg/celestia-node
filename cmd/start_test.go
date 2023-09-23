package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestStart(t *testing.T) {
	testFlag := "StartFlag"
	flags := &pflag.FlagSet{}
	flags.String(testFlag, "", "")

	nodes := []node.Type{node.Light, node.Bridge, node.Full}
	for _, node := range nodes {
		startCmd := Start(flags)

		// Check logic for setting flags
		require.NotNil(t, startCmd.Flags().Lookup(testFlag))

		// Mock command function
		initContext := WithNodeType(context.Background(), node)
		startCmd.RunE = func(cmd *cobra.Command, args []string) error {
			require.Equal(t, cmd.Context(), initContext)
			return nil
		}

		err := startCmd.ExecuteContext(initContext)

		// Check logic for execution command function
		require.NoError(t, err)
	}
}
