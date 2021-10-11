package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	const repoName = "dev"
	devCmd.AddCommand(
		cmd.Start(repoName, node.Dev),
	)
}

var devCmd = &cobra.Command{
	Use:  "dev start",
	Args: cobra.NoArgs,
}
