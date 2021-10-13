package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	devCmd.AddCommand(
		cmd.Start("", node.Dev),
	)
}

var devCmd = &cobra.Command{
	Use:   "dev start",
	Short: "Run a full node in dev mode",
	Long: `Dev mode starts a full-featured Celestia Node alongside a mock embedded Core node process
to simulate block production for testing/development purposes only. Dev mode disables p2p and manages node/chain data 
in memory.`,
	Args: cobra.NoArgs,
}
